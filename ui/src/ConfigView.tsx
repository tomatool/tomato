import React, { FC } from 'react';
import { makeStyles, Theme, createStyles } from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Grid from '@material-ui/core/Grid';
import Typography from '@material-ui/core/Typography';
import Paper from '@material-ui/core/Paper';
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

const ConfigView: FC<{ config: Config, dictionary: Dictionary }> = ({ config, dictionary }) => {
  const classes = useStyles();

  return (
    <Container maxWidth="lg">
      <Typography variant="h5" component="h5" gutterBottom>
        Feature Paths
      </Typography>
      <Grid container spacing={2}>
        <div className={classes.demo}>
          <List dense={true}>
            {config.feature_paths?.map(path => (
              <Link href={`#${path}`} color="inherit">
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
            <br/> 
      <Typography variant="h5" component="h5" gutterBottom>
        Resources
      </Typography>
      <Grid container spacing={2}>
        {config.resources?.map(r => {
          return (
            <Grid item md={3} xs={4}>
              <ConfigResourceView 
                resource={r} 
                resourceOptions={resourceOptions(dictionary)} 
                resourceTypes={resourceTypes(dictionary)} 
                onSave={(r: ConfigResource) => console.log(r)} />
            </Grid>
          );
        })}
      </Grid>
    </Container>
  );
}

export default ConfigView;
