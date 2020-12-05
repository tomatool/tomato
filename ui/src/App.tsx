import React, { useEffect, useState } from 'react';
import axios from 'axios';
import ConfigView from './ConfigView';
import { Config, Dictionary } from './interfaces';



function App() {
  const [config, setConfig] = useState({} as Config);
  const [dictionary, setDictionary] = useState({} as Dictionary);
  // const [feature, setFeature] = useState({} as Feature);
  // this number should represent how many http call required for the loading to be exit
  const [loading, setLoading] = useState(2 as number);

  async function fetchStateFromRemote(path: string, setter: Function) {
    const { data }  = await axios.get(path);
    setter(data);
    setLoading((l) => l-1); 
  }

  useEffect(() => {
    fetchStateFromRemote('/api/config', setConfig);
    fetchStateFromRemote('/api/dictionary', setDictionary);
  },[]);

  useEffect(() => {
    if (window.location.hash.endsWith('.feature')) {

    }
  }, [window.location.hash])
  return (
    <div className="App">
        {loading !== 0 ? `(${loading}) loading...` : <ConfigView config={config} dictionary={dictionary} /> }
    </div>
  );
}

export default App;
