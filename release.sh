#!/bin/bash

set -e

latest=`cat bin/latest_image`
if [ "$latest" == "" ]; then
	echo Cannot find the latest image
	exit -1
fi

echo Latest image is $latest

escaped_latest=${latest//\//\\\/}

yaml=kstat.yaml

files=`find ./deploy/ |grep yaml |sort`
rm -f $yaml
for f in $files
do
	echo $f
	cat $f >> $yaml
	echo --- >> $yaml
done

sed -i "s/image\:\ .*\/${project}:.*/image\:\ ${escaped_latest}/g" $yaml
sed -i "s/-\ .*\/${project}:.*/-\ ${escaped_latest}/g" $yaml

echo Updated kstat.yaml
docker push $latest
