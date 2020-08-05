export interface IResource {
    name: string;
    type: string;
    options: object;
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
   options: Array<IOption>;
}

export interface IOption {
   name: string;
   type: string;
   description: string;
}