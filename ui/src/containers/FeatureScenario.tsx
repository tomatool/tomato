import React from 'react';
import { Input, Select, Form, Button } from 'antd';
import { PlusOutlined, CloseCircleTwoTone } from '@ant-design/icons';
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

  let handleRemove = (e) => {
    handleScenarioItemChange(idx, null);
  }

  const handleAddStepItem = () => {
    const newResourceItem = {
      resource: {
        type: "wiremock",
        name: "tomato-wiremock"
      },
      action: {
        name: "response_path",
        description: "set a response code and body for a given path for wiremock"
      },
      expression: "set $resource with path $path response code to $code and response body $body",
      arguments: [
        {
          "name": "path",
          "type": "string",
          "value": ""
        },
        {
          "name": "code",
          "type": "number",
          "value": ""
        },
        {
          "name": "body",
          "type": "json",
          "value": ""
        }
      ]
    }

    let copy = Object.assign({}, item);
    copy.steps = [...copy.steps, newResourceItem]

    handleScenarioItemChange(idx, copy);
  }


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
        <div style={{ marginTop: "1rem" }}>
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
        </div><br />
        <Form.Item>
          <Button
            style={{ height: '60px' }}
            type="dashed"
            onClick={handleAddStepItem}
            block
          >
            <PlusOutlined /> Add new step
          </Button>
        </Form.Item>
      </td>
      <td valign="top" style={{ padding: '10px' }}>
        <a onClick={handleRemove} href="/#">Remove Scenario</a>
      </td>
    </tr>
  );
}

export default FeatureScenarioContainer;
