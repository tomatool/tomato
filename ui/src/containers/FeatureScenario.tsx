import React from 'react';
import { Input, Select, Form } from 'antd';
import { IDictionary, IScenario, IStep } from '../interfaces';
import { getListOfResources, getResourceOptions, getResourceActions } from '../dictionary';
import ScenarioStepContainer from './ScenarioStep'

const { Option } = Select;

interface IProps {
  dictionary: IDictionary;
  item: IScenario;
  handleScenarioItemChange: (index: number, number: IScenario | null) => void;
  idx: number
  config: any
}

function FeatureScenarioContainer({ dictionary, item, handleScenarioItemChange, idx, config }: IProps) {
  let handleNameChange = (e) => {
    let copy = Object.assign({}, item);
    copy.title = e.target.value;

    handleScenarioItemChange(idx, copy);
  }

  const handleStepItemChange = (index: number, newItem: IStep | null) => {
    if (newItem === null) return handleStepItemRemove(index);
    const newResourceItem = item.steps.map((item: IStep, idxStep: number) => {
      if (idxStep === index) {
        return newItem;
      }
      return item;
    });

    let copy = Object.assign({}, item);
    copy.steps = newResourceItem

    handleScenarioItemChange(idx, copy);
  }

  const handleStepItemRemove = (index: number) => {
    const newResourceItems = item.steps.filter((item: IStep, idxStep: number) => {
      return (idxStep !== index)
    });

    let copy = Object.assign({}, item);
    copy.steps = newResourceItems

    handleScenarioItemChange(idx, copy);
  }
  // let handleTypeChange = (value) => {
  //   let copy = Object.assign({}, item); 
  //   copy.type = value;
  //   copy.options = {};

  //   handleResourceItemChange(item.name, copy);
  // }

  // let handleOptionChange = (e) => {
  //   let copy = Object.assign({}, item);
  //   if(!copy.options) copy.options = {};

  //   copy.options[e.target.name] = e.target.value;

  //   handleResourceItemChange(item.name, copy);
  // }

  // let handleRemove = (e) => {
  //   handleResourceItemChange(item.name, null);
  // }

  // let resourceTypeSelect = (
  //   <Select onChange={handleTypeChange} placeholder="Resource type" defaultValue={item.type}>
  //     {getListOfResources(dictionary).map((resource, index) => {
  //         return (<Option 
  //                   key={index} 
  //                   value={resource} >{resource}</Option>);
  //     })}
  //   </Select>
  // );


  return (
    <tr className="FeatureScenario" style={{ width: '600px', border: 'solid 1px #dfdfdf' }}>
      <td valign="top" style={{ padding: '10px' }}>
        <Form.Item label="">
          <Input.Group compact>
            <Form.Item
              noStyle
              rules={[{ required: true, message: 'Scenario title is required' }]}
            >
              <strong>Scenario Title</strong><br />
              <Input
                onChange={handleNameChange}
                style={{ width: '100%' }}
                value={item.title}
                placeholder="Scenario Title" />
            </Form.Item>
          </Input.Group>
        </Form.Item>
      </td>
      <td style={{ padding: '10px', width: "50rem" }}>
        <strong>Scenario Steps</strong><br />
        <div style={{marginTop: "1rem"}}>
          {item.steps.map((element, index) => {
            return (
              <ScenarioStepContainer
                key={index}
                dictionary={dictionary}
                item={element}
                idx={index}
                config={config}
                handleStepItemChange={handleStepItemChange}
              />
            );
          })}
        </div>
      </td>
      {/* <td valign="top" style={{ padding: '10px'}}>
          {getResourceOptions(dictionary, item.type).map((option, index) => {
            return (
              <Form.Item key={index} style={{ width: '100%' }}>
                <Input
                  name={option.name}
                  onChange={handleOptionChange}   
                  placeholder={option.name}
                  value={(item.options && item.options[option.name]) ? item.options[option.name] : ''}
                  />
                <small>{option.description}</small>
              </Form.Item>
            );
          })}
        </td> */}
      <td valign="top" style={{ padding: '10px' }}>
        <a href="/#">Remove Scenario</a>
      </td>
    </tr>
  );
}

export default FeatureScenarioContainer;
