#!/bin/sh
#
#       /etc/rc.d/init.d/apiserver
#
#       apiserver daemon
#
# chkconfig:   2345 95 05
# description: a apiserver script

### BEGIN INIT INFO
# Provides:       apiserver
# Required-Start:
# Required-Stop:
# Should-Start:
# Should-Stop:
# Default-Start: 2 3 4 5
# Default-Stop:  0 1 6
# Short-Description: apiserver
# Description: apiserver
### END INIT INFO

PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}
DIRECTORY=/home/phuslu/apiserver
SUDO=$(test $(id -u) = 0 || echo sudo)

if [ ! -d "${DIRECTORY}" ]; then
    if command -v realpath >/dev/null; then
        DIRECTORY="$(dirname "$(realpath "$0")")"
    fi
fi
if [ -n "${SUDO}" ]; then
    echo "ERROR: Please run as root"
    exit 1
fi

_start() {
    _check_installed
    test $(ulimit -n) -lt 65535 && ulimit -n 65535
    if ! grep -q bbr /proc/sys/net/ipv4/tcp_congestion_control; then
        if grep -q bbr /proc/sys/net/ipv4/tcp_available_congestion_control; then
            echo bbr > /proc/sys/net/ipv4/tcp_congestion_control
        fi
    fi
    cd ${DIRECTORY}
    nohup env watchdog=1 ${DIRECTORY}/apiserver -log_dir . >/dev/null 2<&1 &
    local pid=$!
    echo -n "Starting apiserver(${pid}): "
    sleep 1
    if (ps ax 2>/dev/null || ps) | grep "${pid} " >/dev/null 2>&1; then
        echo "OK"
    else
        echo "Failed"
    fi
}

_stop() {
    _check_installed
    local pid="$(pidof apiserver)"
    echo -n "Stopping apiserver(${pid}): "
    if pkill -x apiserver; then
        echo "OK"
    else
        echo "Failed"
    fi
}

_restart() {
    if ! ${DIRECTORY}/apiserver -validate >/dev/null 2>&1; then
        echo "Cannot restart apiserver, please correct apiserver toml file"
        echo "Run '${DIRECTORY}/apiserver -validate' for details"
        exit 1
    fi
    _stop
    _start
}

_reload() {
    kill -HUP $(pgrep -o -x apiserver)
}

_install() {
    cp -f ${DIRECTORY}/apiserver.sh /etc/init.d/apiserver
    if command -v systemctl >/dev/null; then
        systemctl enable apiserver
    elif command -v chkconfig >/dev/null; then
        chkconfig apiserver on
    elif command -v update-rc.d >/dev/null; then
        update-rc.d apiserver defaults
    elif command -v rc-update >/dev/null; then
        rc-update add apiserver default
    else
        echo "Unsupported linux system"
        exit 0
    fi
    echo "Install apiserver service OK"
}

_uninstall() {
    if command -v systemctl >/dev/null; then
        systemctl disable apiserver
    elif command -v chkconfig >/dev/null; then
        chkconfig apiserver off
    elif command -v update-rc.d >/dev/null; then
        update-rc.d -f apiserver remove
    elif command -v rc-update >/dev/null; then
        rc-update delete apiserver default
    else
        echo "Unsupported linux system"
        exit 0
    fi
    rm -rf /etc/init.d/apiserver
    echo "Uninstall apiserver service OK"
}

_check_installed() {
    local rcscript=/etc/init.d/apiserver
    if [ -f "${rcscript}" ]; then
        if [ "$0" != "${rcscript}" ]; then
            echo "apiserver already installed as a service, please use systemctl/service command"
            exit 1
        fi
    fi
}

_usage() {
    echo "Usage: [sudo] $(basename "$0") {start|stop|reload|restart|install|uninstall}" >&2
    exit 1
}

case "$1" in
    start)
        _start
        ;;
    stop)
        _stop
        ;;
    restart)
        _restart
        ;;
    reload)
        _reload
        ;;
    install)
        _install
        ;;
    uninstall)
        _uninstall
        ;;
    *)
        _usage
        ;;
esac

exit $?

