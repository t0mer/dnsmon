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



    });
}

function get_monitored_domains_count() {
    $.get("api/monitored", function (data) {
        $('#records_count').text(data.length);
    });
}



