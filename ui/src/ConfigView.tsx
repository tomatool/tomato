import React, {FC} from 'react';
import { Config, Resource } from './interfaces'
import Editor from '@monaco-editor/react';
import yaml from 'yaml';

const ConfigView: FC<Config> = (config: Config) => {  
  const doc = new yaml.Document();
doc.contents = config;
  
  return (
    <div className="Config" style={{ fontSize: '11px', border: 'solid 12px' }}>
      <Editor height={window.innerHeight} language="yaml" value={doc.toString()}/>
    </div>
  );
}

export default ConfigView;
