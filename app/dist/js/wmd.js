$(document).ready(function () {
    get_servers();
    query();
});

function get_servers() {
    $.get("api/servers", function (data) {

        $.each(data, function (i, item) {
             var $tr = $('<tr>').append(
                $('<td>').html("<img class='flag' src='/dist/img/flags/" +item["country"] + ".svg' />"),
                $('<td>').text(item["latitude"]),
                $('<td>').text(item["longitude"]),
                $('<td>').text(item["location"]),
                $('<td>').text(item["provider"]),
                $('<td class="srv" id="' + item["id"] + '" >').html('&nbsp;')
              );
   
            $('#wmd-servers').append('<tr>' + $tr.wrap('<tr>').html() + '</tr>');
          });
        //   $('#loading').hide();

          $('#wmd-servers').DataTable({
            "searching":false,
            "showing": false,
            "bLengthChange" : false,
            "paging": false

        });

    });
}

function get_monitored_domains_count() {
    $.get("api/monitored", function (data) {
        $('#records_count').text(data.length);
    });
}


function query() {
    $(".srv").each(function() {
        var server_id = $(this).attr('id');
        var url = "api/query/?server=" + server_id + "&type=SOA&query=techblog.co.il";

        $.ajax({
            url: url,
            success: function(data) {
                $('#' + server_id).text(data);
            }
        });
    });
}

function getDevices() {
    $.ajax({
        type: "get",
        url: "api/servers",
        dataType: "json",
        success: function (data) {
            if(JSON.stringify(data).includes("error"))
            {
                // alert("error");
                // $('#xia_devices').hide();
                // $('#loading').hide();
            }
            else
            {
                $.each(data, function (i, item) {
                    var $tr = $('<tr>').append(
                        $('<td>').text(item["country"]),
                        $('<td>').text(item["latitude"]),
                        $('<td>').text(item["longitude"]),
                        $('<td>').text(item["location"]),
                        $('<td>').text(item["provider"])
                      );
           
                    $('#wmd-servers').append('<tr>' + $tr.wrap('<tr>').html() + '</tr>');
                  });
                //   $('#loading').hide();
                  $('#wmd-servers').show();
                  $('.tokens').DataTable({
                    dom: 'Bfrtip',
                    buttons: [
                        'copyHtml5',
                        'excelHtml5',
                        'pdfHtml5'
                    ]
                });
                  
            }
            
        }
    });
}