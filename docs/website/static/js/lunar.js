$('.modal').on('show.bs.modal', function (e) {
    if($(e.currentTarget).attr("data-popup")){
        $("body").addClass("body-scrollable");
    }
});
$('.modal').on('hidden.bs.modal', function (e) {
    $("body").removeClass("body-scrollable");
});