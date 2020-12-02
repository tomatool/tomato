import React, { useEffect, useState } from 'react';
import axios from 'axios';
import ConfigView from './ConfigView';
import { Config, Dictionary } from './interfaces';


function App() {
  const [config, setConfig] = useState({} as Config);
  const [dictionary, setDictionary] = useState({} as Dictionary);
  const [loading, setLoading] = useState(0 as number);

  useEffect(() => {
    const initConfig = async () => {
      try {
        const response = await axios.get('/api/config');
        setConfig(response.data);
        setLoading(currentState => currentState+1);
      } catch (e) {
        alert(e);
      }
    };
    initConfig();
  },[]);

  useEffect(() => {
    const initDictionary = async () => {
      try {
        const response = await axios.get('/api/dictionary');
        setDictionary(response.data);
        setLoading(currentState => currentState+1);
      } catch (e) {
        alert(e);
      }
    };
    initDictionary();
  },[]);
  
  return (
    <div className="App">
        {loading < 2 ? `(${loading}) loading...` : <ConfigView config={config} dictionary={dictionary} /> }
    </div>
  );
}

export default App;
