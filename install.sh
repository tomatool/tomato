#!/bin/sh

organization=tomatool
project_name=tomato
system=$(uname -s | awk '{print tolower($0)}')
hardware=$(uname -m)

if [ $hardware = "x86_64" ]
then
  hardware="amd64"
fi

curl -s https://api.github.com/repos/$organization/$project_name/releases/latest \
| grep "browser_download_url.*$system-$hardware" \
| cut -d : -f 2,3 \
| tr -d \" \
| wget -O $project_name.$system-$hardware.tar.gz -qi -

tar -xzvf ./$project_name.$system-$hardware.tar.gz -C /usr/local/bin/
