#!/bin/bash

echo "Testing array range path functionality..."
echo "========================================"

echo -e "\n1. Testing [*] - all array elements:"
./injector -f data2.yaml --set "config.[*].sex=2"

echo -e "\n2. Testing [0..1] - range 0 to 1:"
./injector -f data2.yaml --set "config.[0..1].sex=2"

echo -e "\n3. Testing [0,2] - specific indices:"
./injector -f data2.yaml --set "config.[0,2].sex=2"

echo -e "\n4. Testing [0] - single index:"
./injector -f data2.yaml --set "config.[0].sex=2"

echo -e "\n5. Testing JSON output with [*]:"
./injector -f data2.yaml --set "config.[*].sex=2" -o json

echo -e "\n6. Testing complex field names:"
./injector -f data2.yaml --set "config.[*].user_id=123"

echo -e "\n7. Testing mixed syntax [0..1,2]:"
./injector -f data2.yaml --set "config.[0..1,2].sex=2"

echo -e "\nAll tests completed!" 