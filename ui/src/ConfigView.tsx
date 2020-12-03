import React, { FC, useState, useEffect } from 'react';
import ksuid from 'ksuid';
import { makeStyles, Theme, createStyles } from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Grid from '@material-ui/core/Grid';
import Typography from '@material-ui/core/Typography';
import Link from '@material-ui/core/Link';
import List from '@material-ui/core/List';
import ListItem from '@material-ui/core/ListItem';
import ListItemText from '@material-ui/core/ListItemText';
import ListItemIcon from '@material-ui/core/ListItemIcon';
import InsertDriveFile from '@material-ui/icons/InsertDriveFile';

import ConfigResourceView from './ConfigResourceView';
import { Dictionary, Config, ConfigResource } from './interfaces';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    root: {
      flexGrow: 1,
      maxWidth: 752,
    },
    demo: {
      backgroundColor: theme.palette.background.paper,
    },
    title: {
      margin: theme.spacing(4, 0, 2),
    },
    icon: {
      minWidth: 30
    }
  }),
);

const resourceTypes = (dictionary: Dictionary): string[] => {
  return dictionary?.handlers?.map(h => h.resources).flat()
}

const resourceOptions = (dictionary: Dictionary): Record<string, string[]> => {
  const types = resourceTypes(dictionary);
  let result: Record<string, string[]> = {};
  for (let i = 0; i < types?.length; i++) {
    result[types[i]] = dictionary.handlers?.find((h) => h.resources.indexOf(types[i]) !== -1)?.options.map((o) => o.name) as string[]
  }
  return result as Record<string, string[]>;
}

const ConfigView: FC<{ config: Config, dictionary: Dictionary }> = (props) => {
  const classes = useStyles();

  props.config.resources = props.config.resources.map((r) => {
    return { ...r, ...{ uuid: ksuid.randomSync().string } }
  });
  const [config, setConfig] = useState(props.config);
  const [newUUID, setNewUUID] = useState(ksuid.randomSync().string);

  const onSave = (uuid: string, newConfigResource: ConfigResource) => {
    let newResources = config.resources.map(r => {
      if (r.uuid === uuid) {
        return newConfigResource;
      }
      return r
    });
    if (uuid === newUUID) {
      newResources.push(newConfigResource)
      setNewUUID(ksuid.randomSync().string);
    }
    setConfig({
      ...config, ...{
        resources: newResources
      }
    })
  }

  const onDelete = (uuid: string) => {
    setConfig({
      ...config, ...{ resources: (config.resources.filter(r => r.uuid!==uuid) as ConfigResource[]).filter(r => r !== undefined) }
    })
  }

  return (
    <Container maxWidth="lg">
      <Typography variant="h5" component="h5" gutterBottom>
        Feature Paths
      </Typography>
      <Grid container spacing={2}>
        <div className={classes.demo}>
          <List dense={true}>
            {config.feature_paths?.map(path => (
              <Link key={path} href={`#${path}`} color="inherit">
                <ListItem>
                  <ListItemIcon className={classes.icon}>
                    <InsertDriveFile />
                  </ListItemIcon>
                  <ListItemText primary={path} />
                </ListItem>
              </Link>
            ))}
          </List>
        </div>
      </Grid>
      <br />
      <Typography variant="h5" component="h5" gutterBottom>
        Resources
      </Typography>
      <Grid container spacing={2}>
        {config.resources?.map((r, index) => {
          return (
            <Grid key={index} item md={3} xs={4}>
              <ConfigResourceView
                uuid={r.uuid}
                resource={r}
                editable={false}
                resourceOptions={resourceOptions(props.dictionary)}
                resourceTypes={resourceTypes(props.dictionary)}
                onSave={onSave}
                onDelete={onDelete} />
            </Grid>
          );
        })}
        <Grid key={newUUID} item md={3} xs={4}>
              <ConfigResourceView
                uuid={newUUID}
                editable={true}
                resource={{uuid: newUUID, name: '', type: 'httpclient', options: {}} as ConfigResource}
                resourceOptions={resourceOptions(props.dictionary)}
                resourceTypes={resourceTypes(props.dictionary)}
                onSave={onSave}
                onDelete={onDelete} />
            </Grid>
      </Grid>
    </Container>
  );
}

export default ConfigView;
