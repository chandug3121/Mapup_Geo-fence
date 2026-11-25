#!/usr/bin/env bash
# Creates demo data: four zones in San Francisco, three vehicles and a few
# alert rules. Usage: ./scripts/seed.sh [API_URL]   (default http://localhost:8080)
set -euo pipefail
API="${1:-http://localhost:8080}"

post() {
  curl -fsS -X POST "$API$1" -H 'Content-Type: application/json' -d "$2"
}
id_of() { sed -E 's/.*"(id|alert_id)":"([^"]+)".*/\2/'; }

echo "Seeding $API"

downtown=$(post /geofences '{"name":"Downtown Delivery Zone","description":"Main delivery area for downtown customers","category":"delivery_zone",
  "coordinates":[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]}' | id_of)
civic=$(post /geofences '{"name":"Civic Center Restricted","description":"No commercial vehicles","category":"restricted_zone",
  "coordinates":[[37.7765,-122.4205],[37.7815,-122.4205],[37.7815,-122.4145],[37.7765,-122.4145],[37.7765,-122.4205]]}' | id_of)
bridge=$(post /geofences '{"name":"Bay Bridge Toll Plaza","category":"toll_zone",
  "coordinates":[[37.7880,-122.3920],[37.7920,-122.3920],[37.7920,-122.3860],[37.7880,-122.3860],[37.7880,-122.3920]]}' | id_of)
post /geofences '{"name":"Mission Customer Area","category":"customer_area",
  "coordinates":[[37.7560,-122.4250],[37.7640,-122.4250],[37.7640,-122.4130],[37.7560,-122.4130],[37.7560,-122.4250]]}' > /dev/null

truck=$(post /vehicles '{"vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}' | id_of)
post /vehicles '{"vehicle_number":"CA-7XYZ-221","driver_name":"Maria Lopez","vehicle_type":"van","phone":"+14155550123"}' > /dev/null
post /vehicles '{"vehicle_number":"DL-3C-AB-9087","driver_name":"Arjun Mehta","vehicle_type":"car","phone":"+919876543210"}' > /dev/null

post /alerts/configure "{\"geofence_id\":\"$civic\",\"event_type\":\"both\"}" > /dev/null
post /alerts/configure "{\"geofence_id\":\"$downtown\",\"vehicle_id\":\"$truck\",\"event_type\":\"entry\"}" > /dev/null
post /alerts/configure "{\"geofence_id\":\"$bridge\",\"event_type\":\"entry\"}" > /dev/null

echo "Done: 4 geofences, 3 vehicles, 3 alert rules."
echo "Truck id: $truck — try:"
echo "curl -X POST $API/vehicles/location -H 'Content-Type: application/json' -d '{\"vehicle_id\":\"$truck\",\"latitude\":37.779,\"longitude\":-122.4175,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}'"
