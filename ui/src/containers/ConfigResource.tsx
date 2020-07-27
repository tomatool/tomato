import React from 'react';
import { Input, Select } from 'antd';
import { IDictionary, IResource } from '../interfaces';
import { getListOfResources, getResourceParams } from '../dictionary';

const { Option } = Select;


interface IProps{
  dictionary: IDictionary;
  item: IResource;
  handleResourceItemChange: any;
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

    handleResourceItemChange(item.name, copy);
  }

  let handleParameterChange = (e) => {
    let copy = Object.assign({}, item); 

    copy.parameters[e.target.name] = e.target.value;
    
    handleResourceItemChange(item.name, copy);
  }

  return (
    <div className="Resource">
        <div>
          <Input placeholder="Name" value={item.name} onChange={handleNameChange}/>
          <Select defaultValue={item.type} style={{ width: 120 }} onChange={handleTypeChange}>
              <Option value="">-- select resource --</Option>
              {getListOfResources(dictionary).map((resource, index) => {
                  return (<Option key={index} value={resource}>{resource}</Option>);
              })}
          </Select>
        </div>
        <div>
          <label htmlFor="">Parameters</label>
          {getResourceParams(dictionary, item.type).map((param, index) => {
            return <Input 
                       key={index} 
                       name={param.name} 
                       placeholder={param.name}
                       value={item.parameters[param.name]}
                       onChange={handleParameterChange} 
                    />
          })}            
        </div>
    </div>
  );
}

export default ConfigResourceContainer;
