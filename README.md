# push-server
Server to receive and send notifies to clients.

## Use case
* Irc message notifies
* Notify after long command has ran on remote machine
* Basicly when ever you want notifies from remote machines to your desktop/laptop
or say for your mobile (if someone creates app for it)

## HTTP API
### /regsiter/
This will register new account and returns either new account's token
or message to check email.
```
curl localhost:8080/register/ -d email=<email> -d password=<password>
```

#### Expects
|param|required|type|
|-----|--------|----|
|email|yes|string|
|password|yes|string|

#### Returns
|status|return value|
|------|------------|
|OK|200|
|ERROR|400|

### /retrieve/
This will return account's token. 400 if authentication fails.
```
curl localhost:8080/retrieve/ -d email=<email> -d password=<password>
```

#### Expects
|param|required|type|
|-----|--------|----|
|email|yes|string|
|password|yes|string|

#### Returns
|status|return value|
|------|------------|
|OK|200|
|ERROR|400|

### /pool/
This will return all pushdatas under specified token as JSON.
```
curl localhost:8080/pool/ -d token=<your_token_here>
```

#### Expects
|param|required|type|
|-----|--------|----|
|token|yes|string|

### /push/
This pushes notify
```
curl localhost:8080/push/ -d token=<your_token_here> -d body=<message_body> \
-d title=<message_title>
```

#### Expects
|param|required|type|defualts|
|-----|--------|----|--------|
|token|yes|string||
|title|yes|string||
|body|no|string|empty string|
|url|no|string|empty string|
|priority|no|integer|1|
|timestamp|no|integer|0 - will be set to current time on clients|

#### Returns
|status|return value|
|------|------------|
|OK|200|
|ERROR|400|

#### Note
##### Priority values
|value|meaning|
|-----|-------|
|1|Send to all clients|
|2|Don't make sound on GCM client if TCP client is live|
|3|Don't send to TCP client|

### /gcm/
This regsiters new Google Cloud Messaging client to specified token
```
curl localhost:8080/gcm/ -d token=<token> -d gcmid=<gcmid>
-d title=<message_title>
```

#### Expects
|param|required|type|defualts|
|-----|--------|----|--------|
|token|yes|string||
|gcmid|yes|string||

#### Returns
|status|return value|
|------|------------|
|OK|200|
|ERROR|400|
|Something wen't wrong on server|500|

## TCP clients
TCP clients is used to receive live notifies. To use this feature,
connect to push-server with TCP/TLS connection (default port 9911) and
send your token AND NOTHING ELSE. You'll now receive notifies where
priority != 3.

### Server
Copy the push-serv.conf.def file to push-serv.conf or add the path with -config flag


## Note
Everything except passwords are saved as plain text on the server.
