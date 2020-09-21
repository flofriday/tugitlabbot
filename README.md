# tugitlabbot
A Telegrambot to notify about changes on the TU Wien GitLab repository.

![Screenshot](screenshot.png)
**Try it out:** [https://t.me/tuGitlabBot](https://t.me/tuGitlabBot)

## Run the bot (native)
1) Install golang
2) Run `go build`
3) Run `TELEGRAM_TOKEN=XXXX ./gitlabbot`<br>
Where XXXX is the Token for the Telegrambot

## Run the bot with docker
1) Install docker
2) Build and start the docker container
```
docker build -t tugitlabbot-template .
docker run -e TELEGRAM_TOKEN=XXXX --rm --name tugitlabbot-container tugitlabbot-template
```
Where XXXX is the Token for the Telegrambot

**Note**: Those commands will run the bot in a docker container, however all state will be lost, when you shut down the container. To prevent this you can use docker volumes:
```
docker volume create tugitlabbot-volume
docker build -t tugitlabbot-template .
docker run --env TELEGRAM_TOKEN=XXXX \
      --mount type=volume,source=tugitlabbot-volume,target=/app/data \
      --name tugitlabbot-container tugitlabbot-template \
```