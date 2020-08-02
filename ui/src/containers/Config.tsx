import React, { useState, useEffect } from 'react';
import { Input, List, Button } from 'antd';
import ConfigResourceContainer from './ConfigResource'
import { IDictionary, IResource, IConfig } from '../interfaces'
import { getResourceParams } from '../dictionary'

interface IProps {
    dictionary: IDictionary;
    config: IConfig;
}

function ConfigContainer({ dictionary, config }:IProps) {
  const [configState, setConfig] = useState<IConfig>(config);

  let handleFeaturesPathChange = () => {}

  let handleResourceItemChange = (selectedName: string, newItem: IResource) => {
    const newResourceItem = configState.resources.map((item: IResource) => {
        if (item.name === selectedName) {
            return newItem;
        }
        return item;
    }); 
    
    setConfig({
        features_path: configState.features_path,
        resources: newResourceItem
    });
  }

  let handleNewResourceItem = () => {
    const newResourceItem = {
        name: "new",
        type: "httpclient",
        parameters: {}
    }; 

    getResourceParams(dictionary, "httpclient").forEach((param) => {
      newResourceItem.parameters[param.name] = ""
    }) 
    
    console.log(newResourceItem)

    setConfig({
        features_path: configState.features_path,
        resources: [...configState.resources, newResourceItem]
    });
  }

//   useEffect(() => 

//   ,[configState])

  

  let validateConfig = (config: IConfig) => {
      // validate & return error
  }

  const handleSave = () => {
      
  }  
  
  return (
    <div className="App">
        <label htmlFor="">Features Path</label>
        <List
            itemLayout="horizontal"
            dataSource={configState.features_path}
            renderItem={item => (
            <List.Item>
                <Input placeholder="Features path" value={item}/>
            </List.Item>
            )}
        />

        <label htmlFor="">Resources</label>
        <List
            itemLayout="horizontal"
            dataSource={configState.resources}
            renderItem={item => (
            <List.Item>
                <ConfigResourceContainer 
                    dictionary={dictionary} 
                    item={item} 
                    handleResourceItemChange={handleResourceItemChange} 
                />          
            </List.Item>
            )}
        />
        <Button onClick={handleNewResourceItem} type="primary">
          New Resource
        </Button>
        <Button onClick={handleSave} type="primary">
          Save
        </Button>

        <div>{JSON.stringify(configState)}</div>
    </div>
  );
}

export default ConfigContainer;
