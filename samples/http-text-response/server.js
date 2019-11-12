'use strict';

const express = require('express');

// Constants
const PORT = 8080;
const HOST = '0.0.0.0';

// App
const app = express();
app.get('/*', (req, res) => {
  var status = parseInt(process.env.RESPONSE_CODE)
  if (isNaN(status)) {
    status = 200;
  };
  res.writeHead(status, {'Content-Type': 'plain/text'});
  var body = process.env.RESPONSE_BODY;
  if (!body) {
    body = 'Hello World';
  }
  res.write(body);
  res.end();
});

app.listen(PORT, HOST);
console.log(`Running on http://${HOST}:${PORT}`);
