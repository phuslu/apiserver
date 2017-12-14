#!/bin/bash

set -ex

REVSION=$(git rev-list --count HEAD)
LDFLAGS="-s -w -X main.version=r${REVSION}"

GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}
GOARM=${GOARM:-6}
CGO_ENABLED=${CGO_ENABLED:-$(go env CGO_ENABLED)}

REPO=$(git rev-parse --show-toplevel)
PACKAGE=apiserver
if [ "${CGO_ENABLED}" = "0" ]; then
    BUILDROOT=${REPO}/build/${GOOS}_${GOARCH}
else
    BUILDROOT=${REPO}/build/${GOOS}_${GOARCH}_cgo
fi
STAGEDIR=${BUILDROOT}/stage
OBJECTDIR=${BUILDROOT}/obj
DISTDIR=${BUILDROOT}/dist

if [ "${GOOS}" == "windows" ]; then
    PROM_EXE="${PACKAGE}.exe"
    PROM_STAGEDIR="${STAGEDIR}"
    PROM_DISTCMD="7za a -y -mx=9 -m0=lzma -mfb=128 -md=64m -ms=on"
    PROM_DISTEXT=".7z"
elif [ "${GOOS}" == "darwin" ]; then
    PROM_EXE="${PACKAGE}"
    PROM_STAGEDIR="${STAGEDIR}"
    PROM_DISTCMD="env BZIP=-9 tar cvjpf"
    PROM_DISTEXT=".tar.bz2"
elif [ "${GOARCH:0:3}" == "arm" ]; then
    PROM_EXE="${PACKAGE}"
    PROM_STAGEDIR="${STAGEDIR}"
    PROM_DISTCMD="env BZIP=-9 tar cvjpf"
    PROM_DISTEXT=".tar.bz2"
elif [ "${GOARCH:0:4}" == "mips" ]; then
    PROM_EXE="${PACKAGE}"
    PROM_STAGEDIR="${STAGEDIR}"
    PROM_DISTCMD="env GZIP=-9 tar cvzpf"
    PROM_DISTEXT=".tar.gz"
else
    PROM_EXE="${PACKAGE}"
    PROM_STAGEDIR="${STAGEDIR}/${PACKAGE}"
    PROM_DISTCMD="env XZ_OPT=-9 tar cvJpf"
    PROM_DISTEXT=".tar.xz"
fi

PROM_DIST=${DISTDIR}/${PACKAGE}_${GOOS}_${GOARCH}-r${REVSION}${PROM_DISTEXT}

OBJECTS=${OBJECTDIR}/${PROM_EXE}

SOURCES="${REPO}/apiserver.toml"

case ${GOOS} in
    windows )
        SOURCES="${SOURCES} \
                 ${REPO}/get-apiserver.cmd"
        ;;
    darwin )
        SOURCES="${SOURCES} \
                 ${REPO}/promgui.command \
                 ${REPO}/get-apiserver.sh"
        ;;
    * )
        SOURCES="${SOURCES} \
                 ${REPO}/apiserver.sh \
                 ${REPO}/get-apiserver.sh"
        ;;
esac

build () {
    mkdir -p ${OBJECTDIR}
    env GOOS=${GOOS} \
        GOARCH=${GOARCH} \
        GOARM=${GOARM} \
        CGO_ENABLED=${CGO_ENABLED} \
    go build -v -ldflags="${LDFLAGS}" -o ${OBJECTDIR}/${PROM_EXE} .
}

dist () {
    mkdir -p ${DISTDIR} ${STAGEDIR} ${PROM_STAGEDIR}
    cp ${OBJECTS} ${SOURCES} ${PROM_STAGEDIR}
    if [ "${GOOS}" = "windows" ]; then
        find ${PROM_STAGEDIR} -name "*.toml" -or -name "*.cmd" -exec sed -i 's/$/\r/' {} \;
    fi

    pushd ${STAGEDIR}
    ${PROM_DISTCMD} ${PROM_DIST} *
    popd
}

clean () {
    rm -rf ${BUILDROOT}
}

case $1 in
    build)
        build
        ;;
    dist)
        dist
        ;;
    clean)
        clean
        ;;
    *)
        build
        dist
        ;;
esac
