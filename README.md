# push-server
Server to receive and send notifies to clients.

## Use case
* Irc message notifies
* Notify after long command has ran on remote machine
* Basicly when ever you want notifies from remote machines to your desktop/laptop
or say for your mobile (if someone creates app for it)

## Usage
### Regsiter
```
curl localhost:8080/register/ -d email=<email> -d password=<password>
```
If registeration didnt ocour any errors, this will return your token or say
to check your email if email verification is enabled.
### Retrieve token
```
curl localhost:8080/retrieve/ -d email=<email> -d password=<password>
```
This will return you your token. 404 if authentication fails.
### Pool
```
curl localhost:8080/pool/ -d token=<your_token_here>
```
This will return all pushdatas under specified token as JSON.
### Push
To push notifies:
```
curl localhost:8080/push/ -d token=<your_token_here> -d body=<message_body> \
-d title=<message_title>
```
You can also add unixtimestamp with `-d timestamp=<time>`. This needs to be
valid unix time stamp (integer).

### Server
Copy the push-serv.conf.def file to push-serv.conf or add the path with -config flag


## Note
Everything except passwords are saved as plain text on the server.
