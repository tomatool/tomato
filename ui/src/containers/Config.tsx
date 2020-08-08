import React from 'react';
import { Input, List, Button, Form } from 'antd';
import { PlusOutlined, CloseCircleTwoTone } from '@ant-design/icons';
import ConfigResourceContainer from './ConfigResource'
import { IDictionary, IResource, IConfig } from '../interfaces'

interface IProps {
    dictionary: IDictionary;
    config: IConfig;
    setConfig: (config: IConfig) => void;
}

function ConfigContainer({ dictionary, config, setConfig }:IProps) {
  const handleFeaturePathsChange = (index) => { 
    return (e) => {
      let featurePaths = config.features_path;
      featurePaths[index] = e.target.value;
  
      setConfig({
        features_path: config.features_path.map((val, i) => {
          if (i === index) return e.target.value;
          return val;
        }),
        resources: config.resources
      })
    }
  }
  const handleFeaturePathsDelete = (index) => {
    return () => {
      setConfig({
        features_path: config.features_path.filter((item, i) => index !== i),
        resources: config.resources
      })
    }
  }
  const handleFeaturePathsAdd = () => {
    setConfig({
      features_path: [...config.features_path, `somewhere/features-${config.features_path.length}`],
      resources: config.resources
    })
  }

  const handleResourceItemChange = (selectedName: string, newItem: IResource | null) => {
    if (newItem === null) return handleResourceItemRemove(selectedName);
    const newResourceItem = config.resources.map((item: IResource) => {
        if (item.name === selectedName) {
            return newItem;
        }
        return item;
    }); 
    
    setConfig({
        features_path: config.features_path,
        resources: newResourceItem
    });
  }

  const handleResourceItemRemove = (selectedName: string) => {
    const newResourceItems = config.resources.filter((item: IResource) => {
        return (item.name !== selectedName)
    });
    
    setConfig({
        features_path: config.features_path,
        resources: newResourceItems
    });
  }

  const handleAddResourceItem = () => {
    const newResourceItem = {
        name: `new-${config.resources.length}`,
        type: 'wiremock',
        options: {}
    }; 
    setConfig({
        features_path: config.features_path,
        resources: [...config.resources, newResourceItem]
    });
  }
  
  return (
    <div className="App">
        <h1>Edit <span role="img" aria-label="tomato">🍅</span> Config</h1>
        <div>
          <strong>Features Path</strong>
          <List
              itemLayout="horizontal"
              dataSource={config.features_path}
              renderItem={(item, index) => (
              <List.Item>
                <Input 
                  key={index}
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
          <thead>
            <tr>
              <th>Resources</th>
            </tr>
          </thead>
          <tbody>
          {config.resources.map((item, index) => {
            return (
              <ConfigResourceContainer 
                        key={index}
                        dictionary={dictionary} 
                        item={item} 
                        handleResourceItemChange={handleResourceItemChange} 
                        />
            );
          })}
          </tbody>
          <tfoot>
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
          </tfoot>
        </table>
        <div>{JSON.stringify(config)}</div>
    </div>
  );
}

export default ConfigContainer;
