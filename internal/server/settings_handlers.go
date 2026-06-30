package server

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// settingsView is the data passed to the settings template.
type settingsView struct {
	Title    string
	Settings settings.Settings
	// Errors maps a form field name to its validation message, shown inline.
	Errors map[string]string
}

// allowedLogoTypes maps an accepted image MIME type (as sniffed from the file
// content, not the client-supplied name) to the extension we store it under.
var allowedLogoTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// handleSettings renders the business-profile settings form (FR-5.1).
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "settings", settingsView{
		Title:    "Settings",
		Settings: cfg,
		Errors:   map[string]string{},
	})
}

// handleSettingsSave validates and persists the business profile, handling the
// optional logo upload, then redirects with a flash (post-redirect-get) (FR-5.1).
func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	// Cap the whole request so a huge upload cannot exhaust memory/disk (NFR-S2).
	r.Body = http.MaxBytesReader(w, r.Body, s.deps.Config.MaxUploadBytes)
	f, err := parseMultipartForm(r, 1<<20)
	if err != nil {
		// Most commonly the body exceeded MaxBytesReader; report it kindly.
		s.setFlash(w, flashError, "Could not read the form — the logo may be too large.")
		s.redirect(w, r, "/settings")
		return
	}

	// Start from the stored settings so profile-only fields update and the rest
	// (SMTP, numbering) are preserved.
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	cfg.BusinessName = f.Required("business_name", "Business name")
	cfg.BusinessAddress = f.String("business_address")
	cfg.TaxID = f.String("tax_id")

	// Logo is optional; only replace it when a file is actually uploaded.
	if file, _, ferr := r.FormFile("logo"); ferr == nil {
		defer file.Close()
		name, lerr := s.saveLogo(file)
		var verr *apperr.ValidationError
		switch {
		case lerr == nil:
			cfg.LogoPath = name
		case errors.As(lerr, &verr):
			for field, msg := range verr.Fields {
				f.Errors.Add(field, msg)
			}
		default:
			s.serverError(w, lerr)
			return
		}
	} else if !errors.Is(ferr, http.ErrMissingFile) {
		s.serverError(w, ferr)
		return
	}

	if !f.Valid() {
		s.render(w, r, http.StatusUnprocessableEntity, "settings", settingsView{
			Title:    "Settings",
			Settings: cfg,
			Errors:   f.Errors.Fields,
		})
		return
	}

	if err := s.deps.Settings.SaveProfile(r.Context(), cfg); err != nil {
		s.serverError(w, err)
		return
	}
	s.setFlash(w, flashSuccess, "Business profile saved.")
	s.redirect(w, r, "/settings")
}

// handleSettingsSaveSMTP validates and persists the SMTP delivery settings
// (FR-5.2), then redirects with a flash (post-redirect-get).
func (s *Server) handleSettingsSaveSMTP(w http.ResponseWriter, r *http.Request) {
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	cfg.SMTPHost = f.Required("smtp_host", "SMTP host")
	cfg.SMTPPort = f.Int("smtp_port", 587)
	cfg.SMTPUsername = f.Required("smtp_username", "SMTP username")
	// The password is never rendered back to the page (NFR-4); a blank field
	// means "keep the stored password" rather than clearing it.
	if pw := f.String("smtp_password"); pw != "" {
		cfg.SMTPPassword = pw
	}

	if !f.Valid() {
		s.render(w, r, http.StatusUnprocessableEntity, "settings", settingsView{
			Title:    "Settings",
			Settings: cfg,
			Errors:   f.Errors.Fields,
		})
		return
	}
	if err := s.deps.Settings.SaveSMTP(r.Context(), cfg); err != nil {
		s.serverError(w, err)
		return
	}
	s.setFlash(w, flashSuccess, "SMTP settings saved.")
	s.redirect(w, r, "/settings")
}

// handleSettingsTestSMTP tests an authenticated SMTP session using the submitted
// settings (so they can be verified before saving) and reports the result via a
// flash (FR-5.2).
func (s *Server) handleSettingsTestSMTP(w http.ResponseWriter, r *http.Request) {
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	// Start from stored settings (so a blank password field reuses the saved
	// one), then apply any values typed into the form.
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	if v := f.String("smtp_host"); v != "" {
		cfg.SMTPHost = v
	}
	if v := f.Int("smtp_port", 0); v != 0 {
		cfg.SMTPPort = v
	}
	if v := f.String("smtp_username"); v != "" {
		cfg.SMTPUsername = v
	}
	if v := f.String("smtp_password"); v != "" {
		cfg.SMTPPassword = v
	}
	if err := s.deps.Mailer.TestConnection(r.Context(), cfg); err != nil {
		s.setFlash(w, flashError, "SMTP test failed: "+err.Error())
	} else {
		s.setFlash(w, flashSuccess, "SMTP connection succeeded.")
	}
	s.redirect(w, r, "/settings")
}

// saveLogo validates an uploaded logo by sniffing its content type, then writes
// it into the uploads directory under a stable name, returning that filename. It
// returns an *apperr.ValidationError for an unsupported image type so the caller
// can show the problem inline on the form.
func (s *Server) saveLogo(file multipart.File) (string, error) {
	// Sniff the type from the first 512 bytes (DetectContentType's window).
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	ext, ok := allowedLogoTypes[http.DetectContentType(head[:n])]
	if !ok {
		return "", apperr.NewValidationError().Add("logo", "must be a PNG, JPEG, GIF, or WebP image")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	dir := s.deps.Config.UploadsDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := "logo" + ext
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return name, nil
}
