// JavaScript Document
$(document).ready(function() {

    "use strict";
    
    $(".quick-form").submit(function(e) {
        e.preventDefault();
        var email = $(".email");
        var flag = false;
        if (email.val() == "") {
            email.closest(".form-control").addClass("error");
            email.focus();
            flag = false;
            return false;
        } else {
            email.closest(".form-control").removeClass("error").addClass("success");
            flag = true;
        }
        var dataString = "&email=" + email.val();
        $(".loading").fadeIn("slow").html("Loading...");
        $.ajax({
            type: "POST",
            data: dataString,
            url: "php/quickForm.php",
            cache: false,
            success: function (d) {
                $(".form-control").removeClass("success");
					if(d == 'success') // Message Sent? Show the 'Thank You' message and hide the form
						$('.loading').fadeIn('slow').html('<font color="#48af4b">Mail sent Successfully.</font>').delay(3000).fadeOut('slow');
					else
						$('.loading').fadeIn('slow').html('<font color="#ff5607">Mail not sent.</font>').delay(3000).fadeOut('slow');
                        document.quickform.reset(); 

								  }
        });
        return false;
    });

    $("#reset").on('click', function() {
        $(".form-control").removeClass("success").removeClass("error");
    });

})



