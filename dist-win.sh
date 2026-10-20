#!/bin/bash
set -eo pipefail
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++
export CGO_LDFLAGS="-static-libstdc++ -static-libgcc -Wl,-Bstatic -lstdc++ -lpthread -Wl,-Bdynamic"
export WINDRESFLAGS="-F pe-x86-64"
export BUILD_DIR=./dist/build-win
export TARGET_DIR=./dist/artifacts

exec=$1
build=$1
ver=${1//./,}','

buildMode=${TAKTCHASE_BUILD_MODE:-"0"}

if [[ $# > 1 ]]
then
  exec+='-s'$2
  build+='-snapshot'$2
  ver+=$2
else
  ver+='0'
fi

if [[ $buildMode == "0" || $buildMode == "1" ]]
then

preRC='#include "winuser.h"
          1 VERSIONINFO
          FILEVERSION '$ver'
          FILEFLAGSMASK 0x3fL
          FILEOS 0x40004L
          FILETYPE 0x1L
          FILESUBTYPE 0x0L
          BEGIN
            BLOCK "StringFileInfo"
            BEGIN
              BLOCK "040904b0"
              BEGIN
                VALUE "CompanyName", "TaktChase contributors"
                VALUE "FileDescription", "taktchase'

postRC='"
      VALUE "LegalCopyright", "Original work Copyright (C) Sebastian Krajewski 2018-2026"
      VALUE "ProductName", "taktchase"
      VALUE "ProductVersion", "'$build'"
    END
    END
    BLOCK "VarFileInfo"
    BEGIN
      VALUE "Translation", 0x409, 1200
    END
END
'

mkdir -p $BUILD_DIR

resgen='windres -l 0 '$WINDRESFLAGS' -o '$BUILD_DIR'/taktchase.syso'

resCore=$preRC'-core.dll'$postRC
resTaktChase=$preRC''$postRC
resLauncher=$preRC' launcher'$postRC

$resgen <<< $resCore

go run tools/assets/assets.go ./ $BUILD_DIR/

cp $BUILD_DIR/taktchase.syso taktchase.syso

go build -trimpath -ldflags "-s -w -X 'github.com/shntsk/taktchase/build.VERSION=$build' -X 'github.com/shntsk/taktchase/build.Stream=Release'" -buildmode=c-shared -o $BUILD_DIR/taktchase-core.dll -v -x -tags "exclude_cimgui_glfw exclude_cimgui_sdli"

rm -f taktchase.syso

$resgen <<< $resTaktChase

cp {bass.dll,bass_fx.dll,bassmix.dll,libyuv.dll,SDL3.dll} $BUILD_DIR/

$CC <<< --verbose -O3 -o $BUILD_DIR/taktchase-cli.exe -I. cmain/main_taktchase.c -I$BUILD_DIR/ -L$BUILD_DIR/ -ltaktchase-core $BUILD_DIR/taktchase.syso -municode

$resgen <<< $resLauncher

$CC <<< --verbose -O3 -D LAUNCHER -o $BUILD_DIR/taktchase.exe -I. cmain/main_taktchase.c -I$BUILD_DIR/ -L$BUILD_DIR/ -ltaktchase-core $BUILD_DIR/taktchase.syso -municode

rm $BUILD_DIR/{taktchase.syso,taktchase-core.h}

go run tools/ffmpeg/ffmpeg.go $BUILD_DIR/

fi

if [[ $buildMode == "0" || $buildMode == "2" ]]
then

mkdir -p $TARGET_DIR

go run tools/pack2/pack.go $TARGET_DIR/taktchase-$exec-win.zip $BUILD_DIR/

rm -rf $BUILD_DIR

fi