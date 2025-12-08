#!/bin/bash

# Check if the correct number of arguments is provided
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <startBlockNumber> <endBlockNumber>"
    exit 1
fi

# Assign the arguments to start and end block variables
startBlock=$1
endBlock=$2

# Define the endpoint URL
url='localhost:15480'

# Engine version
version='0.5.1-beta'

# Create a log file with the suffix based on the start and end block numbers
logfile="binary_search_${startBlock}_${endBlock}.log"

# Function to perform the binary search
binary_search() {
    local start=$1
    local end=$2

    # If start equals end, we are checking a single block
    if [ $start -eq $end ]; then
        echo -e "\nTesting block: $start" | tee -a $logfile
    else
        echo -e "\nTesting range: $start - $end" | tee -a $logfile
    fi

    # Execute the curl command and capture the output
    response=$(curl --silent --location --request POST "$url" --header 'Content-Type: application/json' --data-raw '{
        "jsonrpc": "2.0",
        "method": "linea_generateConflatedTracesToFileV2",
        "params": [{"startBlockNumber": '$start',"endBlockNumber": '$end',"expectedTracesEngineVersion": "'$version'"}],
        "id": 1
    }')

    # Log the response
    echo "$response" | tee -a $logfile

    # Check if the response indicates a successful range
    if echo "$response" | grep -q '"result"'; then
        echo "Range $start - $end is successful, skipping further investigation in this range." | tee -a $logfile
        return
    fi

    # If it's a single block, return after the check
    if [ $start -eq $end ]; then
        return
    fi

    # Calculate the middle block
    local mid=$(( (start + end) / 2 ))

    # Recursive binary search on the first half
    binary_search $start $mid

    # Recursive binary search on the second half
    binary_search $((mid + 1)) $end
}

# Start the binary search with the provided arguments
binary_search $startBlock $endBlock
