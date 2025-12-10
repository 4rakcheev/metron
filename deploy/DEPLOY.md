# Server Preparation

## Add user metron, SSH access
TDB

## Add user to sudoers

Put following content to the file `/etc/sudoers.d/metron` 

```shell
metron ALL=(ALL) NOPASSWD: \
    /usr/bin/systemctl restart metron, \
    /usr/bin/systemctl status metron, \
    /usr/bin/systemctl daemon-reload, \
    /usr/bin/chmod, \
    /usr/bin/chown
```

## Setup Service

1. Add Service description from the .service file to the `/etc/systemd/system/metron-bot.service`

2. Create working directories with `metron` user permission:

```shell
mkdir /opt/metron-bot
chown metron:metron /opt/metron-bot
mkdir /opt/metron-bot/logs
chown metron:metron /opt/metron-bot/logs
```
