import React, { useState, useEffect } from 'react'
import { Input, Select, Form, Typography } from 'antd';

const { Text, Link } = Typography;

function Expression({
  expression,
  argument,
  action,
  resource,
}: any) {

  const [step, setStep] = useState(expression)

  const getArgument = (name: string) => {
    return argument.find(ar => ar.name === name)
  }

  const getArgumentValue = (type: string, value: any) => {
    if (type === 'json' || type === 'table') {
      return ('\r\n' + '"""' + '\r\n' + value + '\r\n' + '"""')
    } else {
      return value
    }
  }

  useEffect(() => {
    const parameters = expression.split(" ")
    let copyParameters = parameters
    parameters.map((param: string, idx: number) => {
      if (param.charAt(0) === '$') {
        if (param === '$resource') {
          copyParameters[idx] = getArgumentValue('string', resource)
        }else {
          let ar = getArgument(param.substring(1))
          copyParameters[idx] = ar !== undefined ? getArgumentValue(ar.type, ar.value) : ''
        }
      }
    })

    setStep(copyParameters.join(' '))
  }, [])



  return (
    <Text code>Given {step}</Text>
  )
}

export default Expression;