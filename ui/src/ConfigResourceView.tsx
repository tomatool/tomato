import React, { FC, useEffect, useState } from 'react';
import { makeStyles } from '@material-ui/core/styles';
import Card from '@material-ui/core/Card';
import CardActions from '@material-ui/core/CardActions';
import CardContent from '@material-ui/core/CardContent';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';
import TextField from '@material-ui/core/TextField';
import InputLabel from '@material-ui/core/InputLabel';
import Select from '@material-ui/core/Select';
import MenuItem from '@material-ui/core/MenuItem';

import { ConfigResource } from './interfaces';

const useStyles = makeStyles({
    root: {
        minWidth: 275,
        minHeight: 208,
        position: 'relative',
        transition: '0.2s'
    },
    title: {
        fontSize: 14,
    },
    pos: {
        marginBottom: 12,
    },
    actions: {
        position: 'absolute',
        bottom: 5
    }
});

const ConfigResourceView: FC<{ resource: ConfigResource, resourceTypes: string[], resourceOptions: Record<string, string[]>, onSave: Function }> = (props) => {
    const classes = useStyles();
    const [resource, setResource] = useState(props.resource);
    const [editable, setEditable] = useState(false);
    
    const onSave = () => {
        props.onSave(resource);
        setEditable(false);
    }

    const onTypeChange = (e: React.ChangeEvent<{ value: unknown }>) => setResource({ ...resource, ...{ type: e.target.value as string, options: {} } })
    const onNameChange = (e: React.ChangeEvent<HTMLInputElement>) => setResource({ ...resource, ...{ name: e.target.value } })
    const onOptionChange = (key: string) => {
        return (e: React.ChangeEvent<HTMLInputElement>) => {
            resource.options[key] = e.target.value;
            setResource({ ...resource, ...{ options: resource.options } })
        }
    }

    return (
        <Card className={classes.root} style={editable ? {minHeight: 308} : {}} variant="outlined">
            <CardContent>
                {!editable ? (
                    <Typography className={classes.title} color="textSecondary" gutterBottom>{resource.type}</Typography>
                ) :
                    <div style={{float:'right'}}>
                        <InputLabel style={{ fontSize: 12 }}>Type</InputLabel>
                        <Select onChange={onTypeChange} value={resource.type}>
                            {props.resourceTypes?.map((r) => <MenuItem value={r}>{r}</MenuItem>)}
                        </Select>
                    </div>
                }

                {!editable ? (
                    <Typography variant="h5" component="h2">{resource.name}</Typography>
                ) : <TextField label="Name" value={resource.name} onChange={onNameChange} />}


                <pre style={{ padding: 0, marginTop: 10, fontSize: 12 }}>
                <div>
                    {props.resourceOptions[resource.type].map(key => (
                        <div>
                            {!editable && `${key}: `}
                            {!editable ? 
                                (resource.options && resource.options[key] !== null ? resource.options[key]: '') : 
                                <TextField style={{ fontSize: 9 }} size="small" label={key} value={resource.options[key]} onChange={onOptionChange(key)} />}
                        </div>
                    ))}
                </div>
                </pre>

            </CardContent>
            <CardActions className={classes.actions}>
                <Button size="small" onClick={() => { setEditable(!editable) }}>{!editable ? 'Edit' : 'Cancel'}</Button>
                {editable && (<Button size="small" variant="contained" color="primary" onClick={onSave}>Save</Button>)}
            </CardActions>
        </Card>
    );
}

export default ConfigResourceView;
