(function($) {
    "use strict"; // Start of use strict

    // Smooth scrolling using jQuery easing
    $(document).on('click', 'a.scroll-to-top', function(e) {
        var $anchor = $(this);
        $('html, body').stop().animate({
            scrollTop: ($($anchor.attr('href')).offset().top)
        }, 1000, 'easeInOutExpo');
        e.preventDefault();
    });

    $(function () {
        $('[data-toggle="tooltip"]').tooltip()
    });

    $(".online-status").each(function(){
        const onlineStatusID = "#" + $(this).attr('id');
        $.get( "/user/status?pkey=" + encodeURIComponent($(this).attr('data-pkey')), function( data ) {
            console.log(onlineStatusID + " " + data)
            if(data === true) {
                $(onlineStatusID).html('<i class="fas fa-link text-success"></i>');
            } else {
                $(onlineStatusID).html('<i class="fas fa-unlink"></i>');
            }
        });
    });
    $(function() {
        $('select.device-selector').change(function() {
            this.form.submit();
        });
    });
    $('[data-toggle=confirmation]').confirmation({
        rootSelector: '[data-toggle=confirmation]',
        // other options
    });

    // Handle OAuth2 login button transition
    $(document).on('submit', 'form.login-oauth', function() {
        const $button = $(this).children("button.btn-social")
        $button.prop("disabled", true);
        $button.children("span").children("span.fab").prop('class', 'fa fa-spinner fa-spin');
    });

    // Handle OIDC login button transition
    $(document).on('submit', 'form.login-oidc', function() {
        const $button = $(this).children("button.btn-social")
        $button.prop("disabled", true);
        $button.children("span.fa").prop('class', 'fa');
        $button.children("span.fa").children("span.login-oidc-spn").prop('class', 'fa fa-spinner fa-spin');
    });
})(jQuery); // End of use strict


