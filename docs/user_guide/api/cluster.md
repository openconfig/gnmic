## `GET /api/v1/cluster`

Request gNMIc cluster state and details

Returns gNMIc cluster state and details

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/cluster
    ```
=== "200 OK"
    ```json
    {
        "name": "collectors",
        "number-of-locked-targets": 70,
        "leader": "clab-telemetry-gnmic1",
        "members": [
            {
                "name": "clab-telemetry-gnmic1",
                "api-endpoint": "clab-telemetry-gnmic1:7890",
                "is-leader": true,
                "number-of-locked-nodes": 23,
                "locked-targets": [
                    "clab-lab2-leaf6",
                    "clab-lab5-spine2",
                    "clab-lab4-leaf4",
                    "clab-lab2-leaf8",
                    "clab-lab3-leaf2",
                    "clab-lab5-spine1",
                    "clab-lab1-spine1",
                    "clab-lab2-super-spine2",
                    "clab-lab3-super-spine1",
                    "clab-lab4-spine3",
                    "clab-lab2-spine3",
                    "clab-lab3-leaf7",
                    "clab-lab5-leaf7",
                    "clab-lab5-leaf8",
                    "clab-lab1-spine2",
                    "clab-lab4-leaf8",
                    "clab-lab4-leaf1",
                    "clab-lab4-spine1",
                    "clab-lab2-spine2",
                    "clab-lab3-spine2",
                    "clab-lab1-leaf8",
                    "clab-lab3-leaf8",
                    "clab-lab4-leaf2"
                ]
            },
            {
                "name": "clab-telemetry-gnmic2",
                "api-endpoint": "clab-telemetry-gnmic2:7891",
                "number-of-locked-nodes": 24,
                "locked-targets": [
                    "clab-lab3-leaf6",
                    "clab-lab1-leaf7",
                    "clab-lab2-leaf3",
                    "clab-lab5-leaf5",
                    "clab-lab1-super-spine1",
                    "clab-lab3-leaf5",
                    "clab-lab4-super-spine1",
                    "clab-lab5-leaf6",
                    "clab-lab2-spine1",
                    "clab-lab3-leaf3",
                    "clab-lab4-leaf3",
                    "clab-lab2-leaf4",
                    "clab-lab4-super-spine2",
                    "clab-lab1-spine3",
                    "clab-lab3-leaf4",
                    "clab-lab5-spine4",
                    "clab-lab1-leaf4",
                    "clab-lab2-leaf2",
                    "clab-lab2-super-spine1",
                    "clab-lab4-spine4",
                    "clab-lab5-leaf2",
                    "clab-lab5-leaf4",
                    "clab-lab4-leaf7",
                    "clab-lab1-spine4"
                ]
            },
                {
                "name": "clab-telemetry-gnmic3",
                "api-endpoint": "clab-telemetry-gnmic3:7892",
                "number-of-locked-nodes": 23,
                "locked-targets": [
                    "clab-lab1-leaf5",
                    "clab-lab3-spine3",
                    "clab-lab1-leaf1",
                    "clab-lab2-spine4",
                    "clab-lab1-super-spine2",
                    "clab-lab5-leaf3",
                    "clab-lab4-spine2",
                    "clab-lab1-leaf3",
                    "clab-lab5-spine3",
                    "clab-lab3-super-spine2",
                    "clab-lab2-leaf5",
                    "clab-lab1-leaf2",
                    "clab-lab1-leaf6",
                    "clab-lab4-leaf5",
                    "clab-lab2-leaf7",
                    "clab-lab3-leaf1",
                    "clab-lab2-leaf1",
                    "clab-lab3-spine1",
                    "clab-lab5-leaf1",
                    "clab-lab5-super-spine2",
                    "clab-lab4-leaf6",
                    "clab-lab3-spine4",
                    "clab-lab5-super-spine1"
                ]
            }
        ]
    }
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```

## `GET /api/v1/cluster/members`

Query gNMIc cluster members

Returns a list of gNMIc cluster members with details

=== "Request"
    ```bash
    curl --request GET gnmic-api-address:port/api/v1/cluster/members
    ```
=== "200 OK"
    ```json
    [
        {
            "name": "clab-telemetry-gnmic1",
            "api-endpoint": "http://clab-telemetry-gnmic1:7890",
            "is-leader": true,
            "number-of-locked-nodes": 23,
            "locked-targets": [
                "clab-lab2-spine3",
                "clab-lab5-spine1",
                "clab-lab2-super-spine2",
                "clab-lab4-leaf2",
                "clab-lab4-leaf4",
                "clab-lab5-spine2",
                "clab-lab1-leaf8",
                "clab-lab4-spine1",
                "clab-lab5-leaf7",
                "clab-lab2-spine2",
                "clab-lab3-super-spine1",
                "clab-lab1-spine1",
                "clab-lab3-leaf2",
                "clab-lab3-spine2",
                "clab-lab2-leaf6",
                "clab-lab4-leaf1",
                "clab-lab4-spine3",
                "clab-lab1-spine2",
                "clab-lab2-leaf8",
                "clab-lab3-leaf8",
                "clab-lab5-leaf8",
                "clab-lab3-leaf7",
                "clab-lab4-leaf8"
            ]
        },
        {
            "name": "clab-telemetry-gnmic2",
            "api-endpoint": "http://clab-telemetry-gnmic2:7891",
            "number-of-locked-nodes": 24,
            "locked-targets": [
                "clab-lab1-spine4",
                "clab-lab2-leaf2",
                "clab-lab3-leaf3",
                "clab-lab4-super-spine1",
                "clab-lab5-leaf4",
                "clab-lab1-spine3",
                "clab-lab1-leaf4",
                "clab-lab3-leaf6",
                "clab-lab5-leaf2",
                "clab-lab2-leaf4",
                "clab-lab3-leaf4",
                "clab-lab4-leaf3",
                "clab-lab5-spine4",
                "clab-lab3-leaf5",
                "clab-lab4-super-spine2",
                "clab-lab1-leaf7",
                "clab-lab2-leaf3",
                "clab-lab2-super-spine1",
                "clab-lab5-leaf6",
                "clab-lab2-spine1",
                "clab-lab1-super-spine1",
                "clab-lab4-leaf7",
                "clab-lab4-spine4",
                "clab-lab5-leaf5"
            ]
        },
        {
            "name": "clab-telemetry-gnmic3",
            "api-endpoint": "http://clab-telemetry-gnmic3:7892",
            "number-of-locked-nodes": 23,
            "locked-targets": [
                "clab-lab1-leaf3",
                "clab-lab1-leaf5",
                "clab-lab3-spine4",
                "clab-lab3-spine3",
                "clab-lab1-leaf1",
                "clab-lab1-leaf6",
                "clab-lab2-leaf5",
                "clab-lab4-leaf6",
                "clab-lab5-leaf1",
                "clab-lab5-leaf3",
                "clab-lab5-super-spine2",
                "clab-lab2-spine4",
                "clab-lab5-super-spine1",
                "clab-lab4-spine2",
                "clab-lab3-spine1",
                "clab-lab4-leaf5",
                "clab-lab5-spine3",
                "clab-lab1-super-spine2",
                "clab-lab2-leaf1",
                "clab-lab3-super-spine2",
                "clab-lab3-leaf1",
                "clab-lab1-leaf2",
                "clab-lab2-leaf7"
            ]
        }
    ]
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```

## `GET /api/v1/cluster/leader`

Queries the cluster leader deatils

Returns details of the gNMIc cluster leader.

=== "Request"
    ```bash
    curl --request POST gnmic-api-address:port/api/v1/cluster/leader
    ```
=== "200 OK"
    ```json
    [
        {
            "name": "clab-telemetry-gnmic1",
            "api-endpoint": "http://clab-telemetry-gnmic1:7890",
            "is-leader": true,
            "number-of-locked-nodes": 23,
            "locked-targets": [
                "clab-lab4-leaf8",
                "clab-lab5-leaf8",
                "clab-lab1-spine2",
                "clab-lab3-leaf7",
                "clab-lab4-leaf4",
                "clab-lab2-leaf8",
                "clab-lab2-spine3",
                "clab-lab4-leaf1",
                "clab-lab4-leaf2",
                "clab-lab4-spine3",
                "clab-lab5-spine2",
                "clab-lab1-spine1",
                "clab-lab2-leaf6",
                "clab-lab5-leaf7",
                "clab-lab1-leaf8",
                "clab-lab3-leaf8",
                "clab-lab3-spine2",
                "clab-lab3-super-spine1",
                "clab-lab5-spine1",
                "clab-lab2-super-spine2",
                "clab-lab3-leaf2",
                "clab-lab2-spine2",
                "clab-lab4-spine1"
            ]
        }
    ]
    ```
=== "500 Internal Server Error"
    ```json
    {
        "errors": [
            "Error Text"
        ]
    }
    ```
