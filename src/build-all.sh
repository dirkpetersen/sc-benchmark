#!/usr/bin/env bash

package=$1
if [[ -z "$package" ]]; then
   echo "usage: $0 <package-name>"
   exit 1
fi

# Extract base name without .go extension for output binaries
basename="${package%.go}"

# Ensure bin directory exists
mkdir -p ../bin

platforms=("windows/amd64" "linux/amd64" "darwin/amd64")

for platform in "${platforms[@]}"
do
     platform_split=(${platform//\// })
     GOOS=${platform_split[0]}
     GOARCH=${platform_split[1]}
     output_name="../bin/${basename}-go-${GOOS}-${GOARCH}"
     if [ $GOOS = "windows" ]; then
         output_name+='.exe'
     fi

     env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o $output_name $package
     if [ $? -ne 0 ]; then
         echo 'An error has occurred! Aborting the script execution...'
         exit 1
     fi
done

