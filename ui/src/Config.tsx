import React, { useState } from 'react';
import { Input, List, Button, Select } from 'antd';
import dictionary from './dictionary'

const { Option } = Select;

function handleChange(value) {
  console.log(`selected ${value}`);
}

interface Resource {
    name: string;
    type: string;
    parameters: any;
}

interface Config {
    features_path: string[];
    resources: Resource[];
}

function getListOfResources(dictionary) {
    return dictionary.handlers.map((handler) => {
        return handler.resources.map((resource) => {
            return resource;
        });
    }).flat()
}



function ConfigContainer() {
  const [config, setConfig] = useState<Config>({
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
          }
      ]
  })

  const handleSave = () => {
      setConfig({
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
          }
      ]
  })
  }  
  return (
    <div className="App">
        <label htmlFor="">Features Path</label>
        <List
            itemLayout="horizontal"
            dataSource={config.features_path}
            renderItem={item => (
            <List.Item>
                <Input placeholder="Features path" value={item}/>
            </List.Item>
            )}
        />
        <label htmlFor="">Resources</label>
        <List
            itemLayout="horizontal"
            dataSource={config.resources}
            renderItem={item => (
            <List.Item>
                <Input placeholder="Name" value={item.name}/>
                <Select defaultValue="" style={{ width: 120 }} onChange={handleChange}>
                    <Option value="">-- select resource --</Option>
                    {getListOfResources(dictionary).map((resource, index) => {
                        return (<Option key={index} value={resource}>{resource}</Option>);
                    })}
                </Select>                
            </List.Item>
            )}
        />

        <Button onClick={handleSave} type="primary">
          Save
        </Button>
    </div>
  );
}

export default ConfigContainer;
