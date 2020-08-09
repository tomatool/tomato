import React from 'react';
import { Input, Select, Form, Typography } from 'antd';
import { IDictionary, IResource, IStep } from '../interfaces';
import { getListOfResources, getResourceOptions, getResourceActions } from '../dictionary';

const { Option } = Select;
const { Text, Link } = Typography;
const { TextArea } = Input;

interface IProps {
  dictionary: IDictionary;
  item: IStep;
  handleStepItemChange: (index: number, newItem: IStep | null) => void;
  idx: number
  config: any
}

function ScenarioStepContainer({ dictionary, item, handleStepItemChange, config, idx }: IProps) {
  // let handleNameChange = (e) => {
  //   let copy = Object.assign({}, item); 
  //   copy.name = e.target.value;

  //   handleResourceItemChange(item.name, copy);
  // }

  let getListOfResourceConfig = (config) => {
    return config.resources.map((resource) => {
      return resource
    }).flat();
  }

  let handleTypeChange = (value) => {
    let copy = Object.assign({}, item);
    let resource = value.split(',')
    copy.resource = {
      name: resource[1],
      type: resource[0]
    };

    handleStepItemChange(idx, copy);
  }

  // let handleOptionChange = (e) => {
  //   let copy = Object.assign({}, item);
  //   if(!copy.options) copy.options = {};

  //   copy.options[e.target.name] = e.target.value;

  //   handleResourceItemChange(item.name, copy);
  // }

  // let handleRemove = (e) => {
  //   handleResourceItemChange(item.name, null);
  // }

  let resourceTypeSelect = (
    <Select onChange={handleTypeChange} placeholder="Resource type" defaultValue={item.resource.name}>
      {getListOfResourceConfig(config).map((resource, index) => {
        return (<Option
          key={index}
          value={`${resource.type}, ${resource.name}`}>{resource.name}</Option>);
      })}
    </Select>
  );

  console.log(getListOfResourceConfig(config))

  let actionSelect = (
    <Select onChange={handleTypeChange} placeholder="Resource type" style={{ width: '100%' }} defaultValue={item.action}>
      {getResourceActions(dictionary, item.resource.type).map((action, index) => {
        return (<Option
          key={index}
          value={action.name} >{action.name}</Option>);
      })}
    </Select>
  )

  return (
    <>

      <Form.Item label="">
        <Input.Group compact>
          <Form.Item
            noStyle
            rules={[{ required: true, message: 'Resource Name is required' }]}
          >
            <strong>Resource Name</strong><br />
            {resourceTypeSelect}
          </Form.Item>
        </Input.Group>
      </Form.Item>
      <div>
        <strong>Action</strong><br />
        {actionSelect}
      </div>
      <div style={{ margin: '1rem 0 1rem 0' }}>
        <strong>Expression</strong><br />
        <Text code>Given <span>{item.expression}</span></Text><br />
      </div>
      <div>
        <strong>Arguments</strong><br />
        {item.arguments.map((argument, index) => {
          return (
            <Form.Item key={index} style={{ width: '100%' }}>
              <label>${argument.name}</label>
              {
                argument.type === 'json' ?
                  <TextArea
                    name={argument.name}
                    // onChange={handleOptionChange}   
                    placeholder={argument.name}
                    value={argument.value}
                    autoSize={{ minRows: 3  }}
                  />
                  :
                  <Input
                    name={argument.name}
                    // onChange={handleOptionChange}   
                    placeholder={argument.name}
                    value={argument.value}
                  />
              }
              <small>{argument.type}</small>
            </Form.Item>
          );
        })}
      </div>
      {/* <td valign="top" style={{ padding: '10px' }}>
        <a href="/#">Remove</a>
      </td> */}
    </>
  );
}

export default ScenarioStepContainer;
