import React from 'react';
import { Input, List, Button, Form } from 'antd';
import { PlusOutlined, CloseCircleTwoTone } from '@ant-design/icons';
import FeatureScenarioContainer from './FeatureScenario'
import { IDictionary, IResource, IConfig, IFeature, IScenario } from '../interfaces'

interface IProps {
    dictionary: IDictionary;
    config: IConfig;
    setFeature: (feature: IFeature) => void;
    feature: IFeature
}

function FeatureContainer({ dictionary, config, setFeature, feature }:IProps) {

  function getListOfResourceConfig(config): Array<string> {
    return config.resources.map((resource) => {
         return resource
    }).flat();
 }

  const handleFeaturesChange = (event) => { 
    let newFeature = feature
    newFeature.title = event.target.value

    setFeature(newFeature)
  }

  const handleScenarioItemChange = (index: number, newItem: IScenario | null) => {
    if (newItem === null) return handleScenarioItemRemove(index);
    const newResourceItem = feature.scenarios.map((item: IScenario, idx: number) => {
        if (idx === index) {
            return newItem;
        }
        return item;
    }); 
    
    setFeature({
        title: feature.title,
        scenarios: newResourceItem
    });
  }

  const handleScenarioItemRemove = (index: number) => {
    const newResourceItems = feature.scenarios.filter((item: IScenario, idx: number) => {
        return (idx !== index)
    });
    
    setFeature({
      title: feature.title,
      scenarios: newResourceItems
  });
  }

  // const handleAddResourceItem = () => {
  //   const newResourceItem = {
  //       name: `new-${config.resources.length}`,
  //       type: 'wiremock',
  //       options: {}
  //   }; 
  //   setConfig({
  //       features_path: config.features_path,
  //       resources: [...config.resources, newResourceItem]
  //   });
  // }
  
  return (
    <div className="App">
        <h1>Edit <span role="img" aria-label="tomato">🍅</span> Feature</h1>
        <div>
          <strong>Feature Title</strong><br />
          <Input 
            style={{ width: '300px' }}
            onChange={handleFeaturesChange} 
            placeholder="Features path" 
            value={feature.title} 
          />
        </div>
        <table style={{ width: '100%', marginTop: '1rem'
       }}>
          <thead>
            <tr>
              <th>Scenarios</th>
            </tr>
          </thead>
          <tbody>
          {feature.scenarios.map((item, index) => {
            return (
              <FeatureScenarioContainer 
                  key={index}
                  dictionary={dictionary} 
                  item={item} 
                  idx={index}
                  config={config}
                  handleScenarioItemChange={handleScenarioItemChange} 
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
                    // onClick={handleAddResourceItem}
                    block
                  >
                    <PlusOutlined /> Add new scenario
                  </Button>
                </Form.Item>
              </td>
            </tr>
          </tfoot>
        </table>
        <div>{JSON.stringify(feature)}</div>
    </div>
  );
}

export default FeatureContainer;
