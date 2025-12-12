# ChatGPT generated : )

import csv
import argparse

BATCH_SIZE = 5

def process_range(int1, int2):
    ranges = []
    start = int1
    while start <= int2:
        end = min((start // BATCH_SIZE + 1) * BATCH_SIZE - 1, int2)
        ranges.append(f"{start}-{end}")
        start = end + 1
    ranges.append('###############')
    return ranges

def read_csv(input_file):
    data = []
    with open(input_file, mode='r', newline='') as file:
        reader = csv.reader(file)
        for row in reader:
            data.append(row)
    return data

def write_csv(output_file, data):
    with open(output_file, mode='w', newline='') as file:
        writer = csv.writer(file)
        for row in data:
            writer.writerow(row)

def main(input_file, output_file):
    data = read_csv(input_file)

    processed_data = []
    for row in data:
        if not row:  # Empty row
            processed_data.append(row)
        elif row[0].startswith('#'):  # Comment row
            processed_data.append(row)
        else:
            try:
                int1, int2 = map(int, row[0].split('-'))
                if int1 < int2:
                    sliced_ranges = process_range(int1, int2)
                    for r in sliced_ranges:
                        processed_data.append([r])
                else:
                    print(f"Skipping invalid range: {row}")
            except ValueError:
                print(f"Skipping invalid row: {row}")

    write_csv(output_file, processed_data)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Process a CSV file of ranges.')
    parser.add_argument('input_file', help='The input CSV file containing ranges.')
    parser.add_argument('output_file', help='The output CSV file to write the processed ranges.')

    args = parser.parse_args()
    main(args.input_file, args.output_file)
