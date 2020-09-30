
# Near Go Warchest бот.
Этот сервис динамически поддерживает не более одного  места среди валидаторов NEAR. Он использует интерфейс командной строки [JSON-RPC] (https://docs.near.org/docs/interaction/rpc) и [Near Shell] (https://github.com/near/near-shell/).

## Особенности

- Динамическое поддержание одного места
- Поддержка учетных записей с несколькими делегатами
- ping новой эпохи
- Докер

## Установите Docker
```
sudo apt-get update
sudo apt install docker.io
```

### Docker

```
git clone https://github.com/rozum-dev/near-go-warchest

cd near-go-warchest

sudo docker pull dmytro1rozum/go-warchest:tagname (download docker image)

sudo docker run -dti --restart always --volume $HOME/near/.near-credentials:/root/.near-credentials --name go-warchest --network=host --env-file env2.list -p 9444:9444 dmytro1rozum/go-warchest:latest /dist/go-warchest -accountId <POOL_ID>  -delegatorId <DELEGATOR_ID>

```

> убедитесь, что у вас есть ключи от учетной записи делегата тут `$HOME/.near-credential`.


Это все. Чтобы проверить выполните **sudo docker logs go-warchest -f**, чтобы остановить, выполните **sudo docker rm go-warchest -f**



### Запуск без Docker

Установите или обновите Go. Необходима как минимум 1.13 версия
https://medium.com/@khongwooilee/how-to-update-the-go-version-6065f5c8c3ec

Также нужно установить [Near Shell](https://github.com/near/near-shell/).

Убедитесь, что у вас есть ключи от учетной записи делегата тут `$HOME/.near-credential`.

    git clone https://github.com/rozum-dev/near-go-warchest

    cd near-go-warchest

    set -a
    source env.list
    set +a

    go build go-warchest.go

    ./go-warchest -accountId <YOUR_POOL_ID> -delegatorId <YOUR_DELEGATOR_ID>




