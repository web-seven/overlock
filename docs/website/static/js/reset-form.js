// JavaScript Document
$(document).ready(function() {

    "use strict";
    
    $(".reset-password-form").submit(function(e) {
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
        $(".loading").fadeIn("slow").html("Please wait a minute...");
        $.ajax({
            type: "POST",
            data: dataString,
            url: "php/resetForm.php",
            cache: false,
            success: function (d) {
                $(".form-control").removeClass("success");
					if(d == 'success') // Message Sent? Show the 'Thank You' message and hide the form
						$('.loading').fadeIn('slow').html('<font color="#48af4b">Check your email inbox.</font>').delay(3000).fadeOut('slow');
					    else
						$('.loading').fadeIn('slow').html('<font color="#ff5607">Mail not sent.</font>').delay(3000).fadeOut('slow');
                         document.resetform.reset(); 
								  }
        });
        return false;
    });

    $("#reset").on('click', function() {
        $(".form-control").removeClass("success").removeClass("error");
    });

    /*----------------------------------------------------*/
    /*  Reset Form Validation
    /*----------------------------------------------------*/
    
    $(".reset-password-form").validate({
        rules:{ 
                email:{
                    required: true,
                    email: true,
                },
                messages:{
                        email:{
                            required: "We need your email address to contact you",
                            email: "Your email address must be in the format of name@domain.com"
                        }, 
                    }
        }
    });

})



