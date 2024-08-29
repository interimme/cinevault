package main

import (
	"flag"
	"log"
	"net/http"
)

// Define a string constant containing the HTML for the webpage. This HTML includes a <h1>
// header tag and a JavaScript snippet. The JavaScript makes a GET request to the
// /v1/healthcheck endpoint and writes the response inside the <div id="output"></div> element.
const html = `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
</head>
<body>
<h1>Simple CORS</h1>
<div id="output"></div>
<script>
// JavaScript to execute after the DOM content is fully loaded.
document.addEventListener('DOMContentLoaded', function() {
    // Perform a fetch request to the healthcheck endpoint.
    fetch("http://localhost:4000/v1/healthcheck").then(
        // If the request is successful, write the response text inside the <div>.
        function (response) {
            response.text().then(function (text) {
                document.getElementById("output").innerHTML = text;
            });
        },
        // If the request fails, display the error inside the <div>.
        function(err) {
            document.getElementById("output").innerHTML = err;
        }
    );
});
</script>
</body>
</html>`

// main is the entry point for the application.
func main() {
	// Define a command-line flag for the server address with a default value of ":9000".
	addr := flag.String("addr", ":9000", "Server address")

	// Parse the command-line flags to make them available for use.
	flag.Parse()

	// Log a message indicating that the server is starting with the specified address.
	log.Printf("starting server on %s", *addr)

	// Start the HTTP server on the specified address.
	// The handler function writes the HTML content to the response for every request.
	err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write the HTML content to the HTTP response.
		w.Write([]byte(html))
	}))

	// If ListenAndServe returns an error, log the error and exit the application.
	log.Fatal(err)
}
