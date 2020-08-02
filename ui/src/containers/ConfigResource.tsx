import React from 'react';
import { Input, Select, Form } from 'antd';
import { IDictionary, IResource } from '../interfaces';
import { getListOfResources, getResourceParams } from '../dictionary';

const { Option } = Select;


interface IProps {
  dictionary: IDictionary;
  item: IResource;
  handleResourceItemChange: any;
}

function ConfigResourceContainer({ dictionary, item, handleResourceItemChange, }:IProps) {
  let handleNameChange = (e) => {
    let copy = Object.assign({}, item); 
    copy.name = e.target.value;

    handleResourceItemChange(item.name, copy);
  }

  let handleTypeChange = (value) => {
    let copy = Object.assign({}, item); 
    copy.type = value;
    copy.parameters = {};
       
    handleResourceItemChange(item.name, copy);
  }

  let handleParameterChange = (e) => {
    let copy = Object.assign({}, item); 
    copy.parameters[e.target.name] = e.target.value;

    handleResourceItemChange(item.name, copy);
  }

  let handleRemove = (e) => {
    handleResourceItemChange(item.name, null);
  }
  
  let resourceTypeSelect = (
    <Select onChange={handleTypeChange} placeholder="Resource type" defaultValue={item.type}>
      {getListOfResources(dictionary).map((resource, index) => {
          return (<Option 
                    key={index} 
                    value={resource} >{resource}</Option>);
      })}
    </Select>
  );

  
  return (
    <tr className="ConfigResource" style={{ width: '600px', border: 'solid 1px #dfdfdf'}}>
         <td valign="top" style={{ padding: '10px' }}>
           <Form.Item label="">
             <Input.Group compact>
               <Form.Item
                noStyle
                rules={[{ required: true, message: 'Resource Name is required' }]}
              >
                <Input 
                    onChange={handleNameChange} 
                    addonBefore={resourceTypeSelect} 
                    style={{ width: '100%' }} 
                    value={item.name}
                    placeholder="Resource Name" />
              </Form.Item>
            </Input.Group>
          </Form.Item>
          </td>
          <td valign="top" style={{ padding: '10px'}}>
          {getResourceParams(dictionary, item.type).map((param, index) => {
            return (
              <Form.Item style={{ width: '100%' }}>
                <Input
                  name={param.name}
                  onChange={handleParameterChange}   
                  placeholder={param.name} />
                <small>{param.description}</small>
              </Form.Item>
            );
          })}
        </td>
        <td valign="top" style={{ padding: '10px'}}>
          <a onClick={handleRemove}>Remove</a>
        </td>
    </tr>
  );
}

export default ConfigResourceContainer;
