# push-server
Server to receive and send notifies to clients.

## Use case
* Irc message notifies
* Notify after long command has ran on remote machine
* Basicly when ever you want notifies from remote machines to your desktop/laptop
or say for your mobile (if someone creates app for it)

## Usage
By default, all push data and pool tokens experies on 10 minutes.
### Pool
To register for pooling (receive notifies), you'll need token and key which you
can make up on your own. Just send following request to the server with curl:
```
curl localhost:8080/token/ -d token=<your_token_here> -d key=<your_key_here>
```
Tokens are user speific and no one can pull notifies under token without
the key. But everyone can push notifies for token, so don't spread it if you
dont want to get shizzels on your pool clients.
After that, do the folloing for pooling
```
curl localhost:8080/pool/ -d token=<your_token_here> -d key=<your_key_here>
```
Notice the different url path!
### Push
To push notifies:
```
curl localhost:8080/push/ -d token=<your_token_here> -d body=<message_body> \
-d title=<message_title>
```

### Server
`-h` flag will do the trick


# TODOs:
* TCP client for instant notifies
* SSL
