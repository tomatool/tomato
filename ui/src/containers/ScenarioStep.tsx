import React, { useEffect } from 'react';
import { Input, Select, Form, Typography } from 'antd';
import { IDictionary, IResource, IStep } from '../interfaces';
import { getListOfResources, getResourceOptions, getResourceActions, getActionArguments } from '../dictionary';
import Expression from './Expression'

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

  let handleActionChange = (value) => {
    let copy = Object.assign({}, item);
    let action = value.split(',')
    copy.action = {
      name: action[0],
      description: action[1],
    }
    copy.expression = action[2]
    copy.arguments = getActionArguments(dictionary, item.action.name, item.resource.type).map((arg) => {
      const obj = {
        name: arg.name,
        type: arg.type,
        value: ''
      }
      return obj
    })

    handleStepItemChange(idx, copy);
  }

  let handleOptionChange = (e, index: number) => {
    let copy = Object.assign({}, item);
    let argument = getActionArguments(dictionary, item.action.name, item.resource.type)[index]
    copy.arguments[index] = {
      name: argument.name,
      type: argument.type,
      value: e.target.value
    }

    handleStepItemChange(idx, copy);
  }

  let handleRemove = (e) => {
    handleStepItemChange(idx, null);
  }

  let resourceTypeSelect = (
    <Select onChange={handleTypeChange} placeholder="Resource type" style={{ width: '50%' }} defaultValue={item.resource.name}>
      {getListOfResourceConfig(config).map((resource, index) => {
        return (<Option
          key={index}
          value={`${resource.type}, ${resource.name}`}>{resource.name}</Option>);
      })}
    </Select>
  );

  let actionSelect = (
    <Select onChange={handleActionChange} placeholder="Action" style={{ width: '50%' }} defaultValue={item.action.name}>
      {getResourceActions(dictionary, item.resource.type).map((action, index) => {
        return (<Option
          key={index}
          value={`${action.name}, ${action.description}, ${action.expressions[0]}`}>{action.name}</Option>);
      })}
    </Select>
  )

  return (
    <>
      <h3>Step {idx+1}</h3>
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
        <Text>Given <span>{item.expression}</span></Text><br />
        <Expression expression={item.expression} resource={item.resource.name} argument={item.arguments}/>
      </div>
      <div>
        <strong>Expression Arguments</strong><br />
        {item.arguments.length !== 0 && getActionArguments(dictionary, item.action.name, item.resource.type).map((argument, index) => {
          return (
            <Form.Item key={index} style={{ width: '100%' }}>
              <label>${argument.name}</label>
              {
                argument.type === 'json' ?
                  <TextArea
                    name={argument.name}
                    onChange={(event) => handleOptionChange(event, index)}   
                    placeholder={argument.name}
                    value={(item.arguments && item.arguments[index]) ? item.arguments[index].value : ''}
                    autoSize={{ minRows: 3  }}
                  />
                  :
                  <Input
                    name={argument.name}
                    onChange={(event) => handleOptionChange(event, index)}   
                    placeholder={argument.name}
                    value={(item.arguments && item.arguments[index]) ? item.arguments[index].value : ''}
                  />
              }
              <small>type: {argument.type}</small><br />
              <small>description: {argument.description}</small>
            </Form.Item>
          );
        })}
      </div>
      <td valign="top" style={{ padding: '10px' }}>
        <a onClick={handleRemove} href="/#">Remove Step</a>
      </td>
    </>
  );
}

export default ScenarioStepContainer;
