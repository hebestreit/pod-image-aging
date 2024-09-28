#!/usr/bin/env bash

set -e

INPUT_JSON=$(cat -)

(
  echo -e "NAMESPACE\tNAME\tCONTAINER\tIMAGE\tIMAGE AGE"
  echo $INPUT_JSON | jq -r '
  .items[] |
  .metadata as $meta |
  .spec.containers[] as $container |
  ($meta.annotations["pod-image-aging.hbst.io/status"] ) |
  try (fromjson.containers[] | select(.name == $container.name))?
  | "\($meta.namespace)\t\($meta.name)\t\($container.name)\t\($container.image)\t\(.createdAt // "N/A")"
  ' | while IFS=$'\t' read -r namespace name container image last_updated; do
      # Parse the last_updated date and calculate the current time
      last_updated_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" ${last_updated%Z*} +%s)
      current_epoch=$(date +%s)
      age=$(( current_epoch - last_updated_epoch ))

      # Convert seconds into weeks, days, hours, and minutes
      weeks=$((age / 604800))  # 1 week = 604800 seconds
      days=$(( (age % 604800) / 86400 ))
      hours=$(( (age % 86400) / 3600 ))
      minutes=$(( (age % 3600) / 60 ))

      # Store each row in a temporary format including epoch time for sorting
      echo -e "$namespace\t$name\t$container\t$image\t${weeks} weeks\t"
  done | sort -k5 -rn | awk -F'\t' 'BEGIN {OFS="\t"} {print $1, $2, $3, $4, $5}'
) | column -t -s$'\t'
