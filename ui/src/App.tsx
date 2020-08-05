import React, { useState, useEffect } from 'react';
import axios from 'axios';
import { Spin, Button } from 'antd';
import Config from './containers/Config';
import { IConfig, IDictionary } from './interfaces';

import 'antd/dist/antd.css';

interface IGlobal {
  loading: boolean;
  config: IConfig;
  dictionary: IDictionary;
}

function App() {
  const [global, setGlobal] = useState<IGlobal>({
    loading: true,
    config: { features_path: [], resources: []},
    dictionary: { handlers:[] }
  } as IGlobal);
  const initialData = (window as any).__INITIAL_DATA__ as object;
  
  useEffect(() => {
    const initConfig = async () => {
      try {
        const result = await axios.get(initialData['serverURL'], { headers: {'client': 'true'} });
        
        setGlobal({
          loading: false,
          config: result.data.config,
          dictionary: result.data.dictionary
        });
      } catch (e) {
        alert(e);
      }
    };
    initConfig();
  }, []);

  const setConfig = (config: IConfig) => {
    setGlobal({
      loading: global.loading,
      config: config,
      dictionary: global.dictionary
    })
  }
  
  const handleSave = async () => {
    setGlobal({
      loading: true,
      config: global.config,
      dictionary: global.dictionary
    });

    try {
      const result = await axios.post(initialData['serverURL'], {}, {data: global.config});
      setGlobal({
        loading: false,
        config: global.config,
        dictionary: global.dictionary
      });
    } catch(e) {
      alert(e);
    }
  }

  return (
    <div className="App">
      { global.loading ? <Spin /> : (
        <div>
          <Config dictionary={global.dictionary} config={global.config} setConfig={setConfig} />
          <Button onClick={handleSave}>Save</Button>
        </div>
      ) }
    </div>
  );
}

export default App;
