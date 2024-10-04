#!/usr/bin/env bash

set -e

INPUT_JSON=$(cat -)
export LC_NUMERIC="en_US.UTF-8"

# Initialize associative arrays and counters for total and per-namespace sums
declare -A namespace_sum
declare -A namespace_count
total_sum=0
total_count=0

date
echo ""

TABLE_PODS=""
while IFS=$'\t' read -r namespace name container image last_updated; do
    # Parse the last_updated date and calculate the current time
    last_updated_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" ${last_updated%Z*} +%s 2>/dev/null || echo 0)
    current_epoch=$(date +%s)
    age=$(( current_epoch - last_updated_epoch ))

    # Convert seconds into weeks
    weeks=$((age / 604800))  # 1 week = 604800 seconds

    # Store each row in a temporary format including epoch time for sorting
    TABLE_PODS+="$namespace\t$name\t$container\t$image\t$weeks weeks\n"

    # Track the total sum of weeks for averages
    total_sum=$((total_sum + weeks))
    total_count=$((total_count + 1))

    # Track namespace-specific sums and counts for averages
    namespace_sum["$namespace"]=$((namespace_sum["$namespace"] + weeks))
    namespace_count["$namespace"]=$((namespace_count["$namespace"] + 1))
done < <(
  echo $INPUT_JSON | jq -r '
  .items[] |
  .metadata as $meta |
  .spec.containers[] as $container |
  ($meta.annotations["pod-image-aging.hbst.io/status"] ) |
  try (fromjson.containers[] | select(.name == $container.name))?
  | "\($meta.namespace)\t\($meta.name)\t\($container.name)\t\($container.image)\t\(.createdAt // "N/A")"
')

echo -e "NAMESPACE\tNAME\tCONTAINER\tIMAGE\tIMAGE AGE\n$(echo -e "$TABLE_PODS" | sort -k5 -rn)" | column -t -s$'\t'

# Print averages grouped by namespace
TABLE_NAMESPACES=""
for ns in "${!namespace_sum[@]}"; do
  ns_avg=$(echo "${namespace_sum[$ns]} / ${namespace_count[$ns]}" | bc -l)
  TABLE_NAMESPACES+="$(printf "%s\t%.0f\n" "$ns" "$ns_avg") weeks\n"
done

echo ""
echo -e "NAMESPACE\tIMAGE AGE (avg)\n$(echo -e "$TABLE_NAMESPACES" | sort -k2 -rn )"| column -t -s$'\t'

echo ""
# Print total average across all namespaces
if [ $total_count -gt 0 ]; then
  total_avg="$(echo "scale=1; $total_sum / $total_count" | bc)"
  printf "Overall average: %.2f weeks\n" "$total_avg"
fi
