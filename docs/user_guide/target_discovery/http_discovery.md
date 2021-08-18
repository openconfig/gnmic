
The HTTP target loader can be used to query targets configurations from a remote HTTP server.

It expects a well formatted `application/json` body and a code 200 response.

It supports secure connections, basic authentication using a username and password and/or Oauth2 token based authentication.

#### Configuration
 
``` yaml
loader:
  type: http
  # ressource URL, must include the http(s) schema
  url: 
  # watch interval at which the HTTP endpoint is queried again
  # to determine if a target was added or deleted.
  interval: 60s
  # HTTP request timeout
  timeout: 50s
  # boolean, if true the client does not verify the server certificates
  skip-verify: false
  # path to a certificate authority that will be used to verify the
  # server certificates. Irrelevant if `skip-verify: true`
  ca-file:
  # path to client certificate file
  cert-file:
  # path to client key file
  key-file:
  # username to be used with basic authentication
  username:
  # password to be used with basic authentication
  password:
  # token to be used with Oauth2 token based authentication
  token:
```
