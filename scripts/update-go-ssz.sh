#!/bin/bash

# Script to copy ssz.go files from bazel build folder to appropriate location.
# Bazel builds to bazel-bin/... folder, script copies them back to original folder where target is.
# This script is limited to the proto directory for now.

bazel query 'kind(ssz_gen_marshal, //proto/...)' | xargs bazel build

# Get locations of proto ssz.go files.

file_list=()
while IFS= read -d $'\0' -r file ; do
    file_list=("${file_list[@]}" "$file")
done < <(find -L $(bazel info bazel-bin)/proto -type f -regextype sed -regex ".*ssz\.go$" -print0)

arraylength=${#file_list[@]}
searchstring="/bin/"

# Copy ssz.go files from bazel-bin to original folder where the target is located.

for (( i=0; i<${arraylength}; i++ ));
do
  destination=${file_list[i]#*$searchstring}
  chmod 755 "$destination"
  cp -R -L "${file_list[i]}" "$destination"
done

