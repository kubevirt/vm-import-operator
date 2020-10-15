# Virtual Machine Import metrics

Virtual Machine Import uses Prometheus for metrics reporting. The metrics can be used for real-time monitoring. Virtual Machine Import does not persist its metrics, if a member restarts, the metrics will be reset.

| Name                       | Description                                                                   | Type      | Labels                                   |
|----------------------------|-------------------------------------------------------------------------------|-----------|------------------------------------------|
| kubevirt_vmimport_counter  | The total number of successful/failed/cancelled Virtual Machine imports.     | Counter   | result=successful\|failed\|cancelled     |
| kubevirt_vmimport_duration | Duration, in seconds, of successful/failed/cancelled Virtual Machine imports.| Histogram | result=successful\|failed\|cancelled     |