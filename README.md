The dojo server implements a simple address-exchange server.
A PUT request  to /address/:teamname with a body holding
a JSON object of the form

	{
		"Address": URL
	}

will store a server address for the given team name.

A GET request to /address/ will return an object containing
all the addresses. A GET request to /address/:teamname will
return the address for a given team name.

An entry can be deleted with a DELETE request to /address/:teamname.
