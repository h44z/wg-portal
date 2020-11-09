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
})(jQuery); // End of use strict


