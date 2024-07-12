function check() {
  var checkBox = document.getElementById("checbox");
  var price1 = document.getElementsByClassName("price1");
  var price2 = document.getElementsByClassName("price2");

  for (var i = 0; i < price1.length; i++) {
    if (checkBox.checked == true) {
      price1[i].style.display = "block";
      price2[i].style.display = "none";
    } else if (checkBox.checked == false) {
      price1[i].style.display = "none";
      price2[i].style.display = "block";
    }
  }
}
check();