Sheets Proxy
============

A proxy that reads from Google spreadsheet and returns JSON [][]string

Config
------

Proxy keeps the service account credential in Secret Manager

Deployment
----------

    $  gcloud functions deploy sheets-proxy --entry-point Serve --trigger-http --runtime go111 --set-env-vars SECRET=projects/cr-lab-jzygmunt-2608185428/secrets/worker-svc/versions/latest
