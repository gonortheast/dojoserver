# dojoserver
--
The dojo server implements a simple address-exchange server. A POST request to
/server?token=$teamtoken&url=$url will create a server instance that will be
polled by the dojo server to find its message.

The address and status of a server can be retrieved with a GET request to
/server/$teamnumber, where $teamnumber is the number of the team, derived from
the team token. The response is JSON encoded in the form:

    struct {
    	URL     string
    	Status  string
    }

where URL is the address of the server and Status is "ok" if the server is up
and running and has a message and holds an error message otherwise.

All the servers can be retrieved with a GET request to /server, JSON encoded in
the form:

    map[string] struct {
    	URL     string
    	Status  string
    }

where each entry in the map holds a server entry, keyed by its team number.

A server entry can be deleted by sending a DELETE request to
/server/$teamnumber?token=$teamtoken (the correct token for the team must be
provided).
