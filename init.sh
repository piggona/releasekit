#!/bin/bash
if [ -z $GOPATH ]
then
    echo -e "\$GOPATH is null\n"
    exit 1
fi

cd $GOPATH/src/github.com/piggona/releasekit/vendor/github.com/libgit2/git2go
echo -e "compiling libgit2...\n"
make install

echo -e "compile completed!\n"
exit 0