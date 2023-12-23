$(document).ready(function () {
    get_servers();
    get_monitored_domains_count();
});

function get_servers() {
    $.get("api/servers", function (data) {
        // var data =  JSON.parse(data);
        var totalServers = data.length;

        // Count of servers in each country
        var serversByCountry = {};
        var uniqueCountries = {};

        $.each(data, function (index, server) {
            var country = server.country;
            serversByCountry[country] = (serversByCountry[country] || 0) + 1;
            uniqueCountries[country] = true;
        });
        $('#servers_count').text(totalServers);
        $('#countries_count').text(Object.keys(uniqueCountries).length);

        $.each(data, function (i, item) {
             var $tr = $('<tr>').append(
                $('<td>').html("<img class='flag' src='/dist/img/flags/" +item["country"] + ".svg' />"),
                $('<td>').text(item["latitude"]),
                $('<td>').text(item["longitude"]),
                $('<td>').text(item["location"]),
                $('<td>').text(item["provider"])
              );
   
            $('#wmd-servers').append('<tr>' + $tr.wrap('<tr>').html() + '</tr>');
          });
        //   $('#loading').hide();

          $('#wmd-servers').DataTable({
            "searching":false,
            "showing": false,
            "bLengthChange" : false,

        });

    });
}

function get_monitored_domains_count() {
    $.get("api/monitored", function (data) {
        $('#records_count').text(data.length);
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