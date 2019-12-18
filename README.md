# Tableau-Exporter-Prometheus
Export Tableau metrics for Prometheus. It's more than inspired of https://community.tableau.com/docs/DOC-5592.

## Expose tableau DB
-	Enable access to the Tableau Server repository: https://help.tableau.com/current/server/en-us/perf_collect_server_repo.htm
-	For windows, open the port 8060 in the windows server firewall: https://community.tableau.com/thread/192653.
-	Try to connect from your tableau desktop running on another machine by adding a new data source: https://help.tableau.com/current/server/en-us/perf_collect_server_repo.htm (Connect to the Tableau Server repository).

## Run with docker
```
docker run --rm -p 9030:9030 -e DATABASE_HOST=tableauServer -e DATABASE_PASSWORD=tableauPostgreSQLUser -e DATABASE_PASSWORD=tableauPostgreSQLPassword alexvaut:promextableau
```
## Build the docker image
```
docker build -t tableau_exporter .
```
## Configuration
```yaml
database:
  host: "localhost"
  port: 8060
  name: "workgroup"
  user: "readonly"
  password: "password"

scrapeIntervalSeconds: 5
port: 9030
```
All the configuration parameters can be setup through environment variables. For instance, for database->host, setup the environment variable DATABASE_HOST.


## Metrics:
Accessible from http://localhost:9030/metrics when publishing port 9030.
```
# HELP tableau_hits_total The total number of hits per project/workbook/view.
# TYPE tableau_hits_total counter
# HELP tableau_server_hits_total The total number of hits on tableau server.
# TYPE tableau_server_hits_total counter
# HELP tableau_server_sessions_total The total number of sessions on tableau server.
# TYPE tableau_server_sessions_total counter
# HELP tableau_server_users_count The number of distinct users on the tableau server on a period of time.
# TYPE tableau_server_users_count gauge
# HELP tableau_session_duration_seconds The total duration of a session.
# TYPE tableau_session_duration_seconds histogram
# HELP tableau_users_count The number of distinct users per project/workbook/view on a specific period of time.
# TYPE tableau_users_count gauge
# HELP tableau_response_time_seconds The time to answer user request per project/workbook/view.
# TYPE tableau_response_time_seconds histogram
```
