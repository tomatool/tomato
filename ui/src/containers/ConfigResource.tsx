import React from 'react';
import { Input, Select, Form } from 'antd';
import { IDictionary, IResource } from '../interfaces';
import { getListOfResources, getResourceOptions } from '../dictionary';

const { Option } = Select;


interface IProps {
  dictionary: IDictionary;
  item: IResource;
  handleResourceItemChange: (selectedName: string, newItem: IResource | null) => void;
}

function ConfigResourceContainer({ dictionary, item, handleResourceItemChange }:IProps) {
  let handleNameChange = (e) => {
    let copy = Object.assign({}, item); 
    copy.name = e.target.value;

    handleResourceItemChange(item.name, copy);
  }

  let handleTypeChange = (value) => {
    let copy = Object.assign({}, item); 
    copy.type = value;
    copy.options = {};
       
    handleResourceItemChange(item.name, copy);
  }

  let handleOptionChange = (e) => {
    let copy = Object.assign({}, item);
    if(!copy.options) copy.options = {};
    
    copy.options[e.target.name] = e.target.value;

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
        </td>
        <td valign="top" style={{ padding: '10px'}}>
          <a onClick={handleRemove} href="/#">Remove</a>
        </td>
    </tr>
  );
}

export default ConfigResourceContainer;
