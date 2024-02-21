$(function () {
    $('.spinner').each(function () {
        var el = $(this);
        var add = el.next('.spinner-add');
        var sub = el.prev('.spinner-sub');

        // subtract
        sub.on('click', function () {
            if (el.val() > parseInt(el.attr('min'))) {
                el.val(function (i, oldval) {
                    return --oldval;
                });
            }
            // disabled
            if (el.val() == parseInt(el.attr('min'))) {
                sub.addClass('disabled');
            }
            if (el.val() < parseInt(el.attr('max'))) {
                add.removeClass('disabled');
            }
        });

        // increment
        add.on('click', function () {
            if (el.val() < parseInt(el.attr('max'))) {
                el.val(function (i, oldval) {
                    return ++oldval;
                });
            }
            // disabled
            if (el.val() > parseInt(el.attr('min'))) {
                sub.removeClass('disabled');
            }
            if (el.val() == parseInt(el.attr('max'))) {
                add.addClass('disabled');
            }
        });
    });
});
