import React, { useState } from 'react';
import { Input, List, Button, Form } from 'antd';
import { PlusOutlined, CloseCircleTwoTone } from '@ant-design/icons';
import ConfigResourceContainer from './ConfigResource'
import { IDictionary, IResource, IConfig } from '../interfaces'

interface IProps {
    dictionary: IDictionary;
    config: IConfig;
}

function ConfigContainer({ dictionary, config }:IProps) {
  const [configState, setConfig] = useState<IConfig>(config);

  const handleFeaturePathsChange = (index) => { 
    return (e) => {
      let featurePaths = configState.features_path;
      featurePaths[index] = e.target.value;
  
      setConfig({
        features_path: configState.features_path.map((val, i) => {
          if (i === index) return e.target.value;
          return val;
        }),
        resources: configState.resources
      })
    }
  }
  const handleFeaturePathsDelete = (index) => {
    return () => {
      setConfig({
        features_path: configState.features_path.filter((item, i) => index !== i),
        resources: configState.resources
      })
    }
  }
  const handleFeaturePathsAdd = () => {
    setConfig({
      features_path: [...configState.features_path, `somewhere/features-${configState.features_path.length}`],
      resources: configState.resources
    })
  }

  const handleResourceItemChange = (selectedName: string, newItem: IResource | null) => {
    if (newItem === null) return handleResourceItemRemove(selectedName);
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

  const handleResourceItemRemove = (selectedName: string) => {
    const newResourceItems = configState.resources.filter((item: IResource) => {
        return (item.name !== selectedName)
    });
    
    setConfig({
        features_path: configState.features_path,
        resources: newResourceItems
    });
  }

  const handleAddResourceItem = () => {
    const newResourceItem = {
        name: `new-${configState.resources.length}`,
        type: "wiremock",
        parameters: {}
    }; 
      
    setConfig({
        features_path: configState.features_path,
        resources: [...configState.resources, newResourceItem]
    });
  }


  const handleSave = () => {
      
  }  
  
  return (
    <div className="App">
        <h1>Edit <span role="img" aria-label="tomato">🍅</span> Config</h1>
        <div>
          <strong>Features Path</strong>
          <List
              itemLayout="horizontal"
              dataSource={configState.features_path}
              renderItem={(item, index) => (
              <List.Item>
                <Input 
                  style={{ width: '300px' }}
                  onChange={handleFeaturePathsChange(index)} 
                  placeholder="Features path" 
                  value={item} 
                  addonAfter={
                    <CloseCircleTwoTone name={item} onClick={handleFeaturePathsDelete(index)} />}
                />
              </List.Item>
              )}
          />
          <Button
              style={{ height: '30px' }}
              onClick={handleFeaturePathsAdd}
            >
              <PlusOutlined /> 
          </Button>
        </div>
        <table style={{ width: '100%' }}>
          <tr>
            <th>Resources</th>
          </tr>
          {configState.resources.map((item, index) => {
            return (
              <ConfigResourceContainer 
                        key={index}
                        dictionary={dictionary} 
                        item={item} 
                        handleResourceItemChange={handleResourceItemChange} 
                        />
            );
          })}
          <tr>
            <td colSpan={3}>
              <Form.Item>
                <Button
                  style={{ height: '60px' }}
                  type="dashed"
                  onClick={handleAddResourceItem}
                  block
                >
                  <PlusOutlined /> Add new resource
                </Button>
              </Form.Item>
            </td>
          </tr>
        </table>
          
        <Button onClick={handleSave} type="primary">
          Save
        </Button>

        <div>{JSON.stringify(configState)}</div>
    </div>
  );
}

export default ConfigContainer;
