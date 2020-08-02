import React, {useState,useEffect} from 'react';
import axios from 'axios';
import Config from './containers/Config';
import 'antd/dist/antd.css';
import dictionary from './dictionary';
import {IConfig} from './interfaces';

function App() {
  const [config, setConfig] = useState<IConfig>({features_path:[],resources:[]});
  const initialData = (window as any).__INITIAL_DATA__;
  
  useEffect(() => {
    axios.get(initialData.serverURL, { headers: {'client': 'true'}})
    .then(function (response) {
      return response.data.config as IConfig;
    })
    .then(function (config: IConfig) {
      setConfig(config);
    })
    .catch(function (error) {
      alert('failed to fetch config file: ' + error)
    })
  }, [config]);

  return (
    <div className="App">
      <Config dictionary={dictionary} config={config} />
    </div>
  );
}

export default App;
