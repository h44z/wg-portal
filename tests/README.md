# pyswagger unittests for the API & UI

## Requirements

```
wg-quick up conf/wg-example0.conf
sudo LOG_LEVEL=debug CONFIG_FILE=conf/config.yml ../dist/wg-portal-amd64 

python3 -m venv ~/venv/apitest
~/venv/apitest/bin/pip install pyswagger mechanize requests pytest PyYAML
```

## Running

### API
```
~/venv/apitest/bin/python3 -m unittest test_API.TestAPI 
```

### UI
```
~/venv/lsl/bin/pytest pytest_UI.py 
```


## Debugging
Debugging for requests http request/response is included for the API unittesting.
To use, adjust the log level for "api" logger to DEBUG

```python
log.setLevel(logging.DEBUG)
<action>
log.setLevel(logging.INFO)
```
This will provide:
```
2021-09-29 14:55:15,585 DEBUG api HTTP
---------------- request ----------------
GET http://localhost:8123/api/v1/provisioning/peers?Email=test%2Bn4gbm7%40example.org
User-Agent: python-requests/2.26.0
Accept-Encoding: gzip, deflate
Accept: application/json
Connection: keep-alive
Authorization: Basic d2dAZXhhbXBsZS5vcmc6YWJhZGNob2ljZQ==

None
---------------- response ----------------
200 OK http://localhost:8123/api/v1/provisioning/peers?Email=test%2Bn4gbm7%40example.org
Content-Type: application/json; charset=utf-8
Date: Wed, 29 Sep 2021 12:55:15 GMT
Content-Length: 285

[{"PublicKey":"hO3pxnft/8QL6nbE+79HN464Z+L4+D/JjUvNE+8LmTs=",
"Identifier":"Test User (Default)","Device":"wg-example0","DeviceIdentifier":"example0"},
{"PublicKey":"RVS2gsdRpFjyOpr1nAlEkrs194lQytaPHhaxL5amQxY=",
"Identifier":"debug","Device":"wg-example0","DeviceIdentifier":"example0"}]
```


