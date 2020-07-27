import React from 'react';
import './App.css';
import Config from './containers/Config';
import 'antd/dist/antd.css';
import dictionary from './dictionary';

function App() {
  let config = {
      features_path: [
          "test.feature"
      ],
      resources: [
          {
            name: "http-cli",
            type: "httpclient",
            parameters: {
                base_url: "http://example.com"
            }
          },
          {
            name: "shell",
            type: "shell",
            parameters: {}
          }
      ]
  }
    

  return (
    <div className="App">
      <Config dictionary={dictionary} config={config} />
    </div>
  );
}

export default App;
