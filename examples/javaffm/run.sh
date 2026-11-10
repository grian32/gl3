#!/usr/bin/env bash

cp ../../gl3 .

case "$(uname -s)" in
    Darwin)
        LIB_NAME="libadd.dylib"
        ;;
    Linux)
        LIB_NAME="libadd.so"
        ;;
    *)
        exit 1
        ;;
esac

./gl3 build --shared -o "$LIB_NAME" add.gl3

javac Main.java
jar cfe app.jar Main Main.class
java --enable-native-access=ALL-UNNAMED -Djava.library.path=. -jar app.jar

rm -f Main.class app.jar gl3 "$LIB_NAME"
