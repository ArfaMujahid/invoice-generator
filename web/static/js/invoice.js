// invoice.js drives the invoice editor: adding/removing line-item rows and
// recalculating per-row and overall totals live as the user types (FR-2.2,
// FR-2.3). It is plain, dependency-free JavaScript (SRS §2.5).
(function () {
  "use strict";

  var table = document.getElementById("line-items");
  if (!table) return;
  var tbody = table.querySelector("tbody");
  var tmpl = document.getElementById("li-template");
  var addBtn = document.getElementById("add-line");
  var taxInput = document.getElementById("tax_rate");

  // num parses a numeric input value, treating blanks/garbage as 0.
  function num(v) {
    var n = parseFloat(v);
    return isNaN(n) ? 0 : n;
  }

  // money formats a number with two decimal places.
  function money(n) {
    return n.toFixed(2);
  }

  // recalc updates every row total and the subtotal/tax/grand-total footer.
  function recalc() {
    var subtotal = 0;
    tbody.querySelectorAll(".li-row").forEach(function (row) {
      var qty = num(row.querySelector(".li-qty").value);
      var price = num(row.querySelector(".li-price").value);
      var lineTotal = qty * price;
      subtotal += lineTotal;
      row.querySelector(".li-total").textContent = money(lineTotal);
    });
    var rate = num(taxInput.value);
    var tax = (subtotal * rate) / 100;
    document.getElementById("subtotal").textContent = money(subtotal);
    document.getElementById("tax").textContent = money(tax);
    document.getElementById("grand-total").textContent = money(subtotal + tax);
  }

  // addRow appends a fresh line-item row cloned from the <template>.
  function addRow() {
    tbody.appendChild(tmpl.content.cloneNode(true));
    recalc();
  }

  // Recalculate on any input within the editor.
  table.addEventListener("input", recalc);
  if (taxInput) taxInput.addEventListener("input", recalc);

  // Remove a row when its remove button is clicked (keeping at least one row).
  tbody.addEventListener("click", function (e) {
    if (!e.target.classList.contains("li-remove")) return;
    var rows = tbody.querySelectorAll(".li-row");
    if (rows.length > 1) {
      e.target.closest(".li-row").remove();
    } else {
      // Clear the last remaining row rather than deleting it.
      e.target.closest(".li-row").querySelectorAll("input").forEach(function (i) {
        i.value = i.classList.contains("li-qty") ? "1" : i.classList.contains("li-price") ? "0.00" : "";
      });
    }
    recalc();
  });

  if (addBtn) addBtn.addEventListener("click", addRow);

  recalc();
})();
