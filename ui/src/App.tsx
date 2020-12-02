import React, { useEffect, useState } from 'react';
import axios from 'axios';
import { Config } from './interfaces';
import ConfigView from './ConfigView';

function App() {
  const [config, setConfig] = useState({} as Config);

  useEffect(() => {
    const initConfig = async () => {
      try {
        const config = await axios.get('/api/config');
        setConfig(config.data);
      } catch (e) {
        alert(e);
      }
    };
    initConfig();
  },[])
  
  return (
    <div className="App">
        <ConfigView {...config}   />
    </div>
  );
}

export default App;
