$(document).ready(function () {
    get_record_types();
    get_servers();
    
    $('#search').click(function(){
        query();
    });

    
});

function get_servers() {
    $.get("api/servers", function (data) {

        $.each(data, function (i, item) {
             var $tr = $('<tr>').append(
                $('<td class="vmiddle">').html("<img class='flag' src='/dist/img/flags/" +item["country"] + ".svg' />"),
                $('<td class="vmiddle">').text(item["latitude"]),
                $('<td class="vmiddle">').text(item["longitude"]),
                $('<td class="vmiddle">').text(item["location"]),
                $('<td class="vmiddle">').text(item["provider"]),
                $('<td  class="vmiddle" id=status_' + item["id"] + ' >').html('&nbsp;'),
                $('<td class="srv vmiddle" id="' + item["id"] + '" >').html('&nbsp;')
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
        var url = "api/query/?server=" + server_id + "&type=" +$('#query-type').val() + "&query=" + $('#query').val();

        $.ajax({
            url: url,
            success: function(data) {
                data = JSON.parse(data);
                console.log(data);
                $('#' + server_id).text(data.answer);
                $('#status_' + server_id).text(data.status);
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

function get_record_types(){
    $.ajax({
        url: 'api/types',
        method: 'GET',
        dataType: 'json',
        success: function(data) {
          var selectBox = $('#query-type');
          
          // Loop through the JSON data and append options to the select box
          $.each(data, function(index, value) {
            selectBox.append($('<option>', {
              value: value,
              text: value
            }));
          });
          selectBox.find('option:first').prop('selected', true);
        },
        error: function(xhr, status, error) {
          console.error(status + ': ' + error);
        }
      }); 
}