# organisation Chart Tool

A command-line tool for managing and processing organisational structure data, specifically designed for tracking government ministries, departments, and personnel appointments.

## Overview

This tool processes transaction data to build and maintain an organisational chart that tracks:
- Government ministries and their relationships
- Department structures under ministries
- Personnel appointments and their changes over time
- Historical tracking of organisational changes

## Features

- **Data Processing**: Processes transaction files from a specified directory to build the organisational structure
- **Entity Management**: 
  - Creates and manages government entities
  - Tracks ministry appointments
  - Manages department structures
  - Handles personnel assignments
- **Relationship Tracking**:
  - Maintains hierarchical relationships between entities
  - Tracks appointment dates and durations
  - Records historical changes in organisational structure
- **API Integration**:
  - Separate endpoints for updates and queries
  - RESTful API interface for data operations
- **Process Types**:
  - Organisation mode: Processes minister and department entities
  - People mode: Processes citizen entities

## Building the Tool

To build the executable from the base directory:

```bash
go build -o orgchart cmd/main.go
```

This will create an executable named `orgchart` in the current directory.

## Usage

The tool can be run with various options:

```bash
# Show help and usage information
./orgchart --help

# Process organisation data with default settings
./orgchart -data /path/to/data/directory

# Process people data
./orgchart -data /path/to/data/directory -type people

# Initialize database and process organisation data
./orgchart -data /path/to/data/directory -init

# Use custom API endpoints
./orgchart -data /path/to/data/directory -update_endpoint http://custom:8080/entities -query_endpoint http://custom:8081/v1/entities
```

### Command Line Options

- `-data`: (Required) Path to the data directory containing transactions
- `-init`: (Optional) Initialize the database with government node
- `-type`: (Optional) Type of data to process: 'organisation' or 'people' (default: organisation)
- `-update_endpoint`: (Optional) Endpoint for the Update API (default: "http://localhost:8080/entities")
- `-query_endpoint`: (Optional) Endpoint for the Query API (default: "http://localhost:8081/v1/entities")

### Process Types

The tool supports two modes of operation:

1. **organisation Mode** (default):
   - Processes minister and department entities
   - Tracks organisational structure
   - Manages hierarchical relationships

2. **People Mode**:
   - Processes citizen entities
   - Tracks personnel appointments
   - Manages individual relationships

## Data Structure

The tool processes transaction files that define:
1. **Ministries**: Government ministries and their appointments
2. **Departments**: organisational units under ministries
3. **Personnel**: People appointed to various positions
4. **Relationships**: Hierarchical and appointment relationships between entities

### Transaction File Naming Convention

Transaction files must follow a specific naming convention:
- Files must contain `_ADD` in their name to be recognized as ADD transactions
- The `_ADD` can be at the end of the filename or preceded by a prefix
- Valid examples:
  - `ADD.csv`
  - `2403-38_ADD.csv`
  - `Xpr_ADD.csv`
  - `2024_03_ADD.csv`

The tool will process all CSV files in the specified directory that match this naming pattern.

## API Endpoints

The tool uses two main API endpoints:
1. **Update API**: Handles all write operations (default: http://localhost:8080/entities)
2. **Query API**: Handles all read operations (default: http://localhost:8081/v1/entities)

## Requirements

- Go 1.x or higher
- Access to the required API endpoints
- Transaction data in the specified format
- CSV files following the required naming convention

## Insert Data

### Insert Minister Department

Note that when you give the directory path it must be the directory in which 
the `ADD`, `MOVE`, `MERGE`, `RENAME` and `TERMINATE` csv files are located. 

### Adding Organization Data for OrgChart

```bash
./orgchart -data $(pwd)/data/orgchart/akd/2024-09-27/ -init true
```

### Adding Person Data for OrgChart

```bash
./orgchart -data $(pwd)/data/people/akd/2024-09-25/ -type person
```


## Development

The project structure:
```
.
├── cmd/
│   └── main.go         # Main application entry point
├── api/                # API client and operations
├── models/             # Data models and structures
└── tests/              # Test files
```

## License

[Add your license information here]

