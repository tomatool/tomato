export interface IResource {
    name: string;
    type: string;
    parameters: any;
}

export interface IConfig {
    features_path: string[];
    resources: IResource[];
}

export interface IDictionary {
   handlers: Array<IHandler>
}

export interface IHandler {
   name: string;
   description: string;
   resources: Array<string>;
   options: Array<IParameter>;
}

export interface IParameter {
   name: string;
   type: string;
   description: string;
}