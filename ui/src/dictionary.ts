import { IDictionary, IOption, IAction } from './interfaces';

export function getListOfResources(dictionary: IDictionary): Array<string> {
   return dictionary.handlers.map((handler) => {
        return handler.resources.map((resource) => {
            return resource;
        });
   }).flat();
}

export function getResourceOptions(dictionary: IDictionary, resourceType: string): Array<IOption> {
   const handler = dictionary.handlers.find(handler => {
        const selectedHandler = handler.resources.find((resource) => resource === resourceType);
         
        if (selectedHandler) return true;

        return false;
   });

   if (handler === undefined) return [];

   return handler.options;
}

export function getResourceActions(dictionary: IDictionary, resourceType: string): Array<IAction> {
   const handler = dictionary.handlers.find(handler => {
        const selectedHandler = handler.resources.find((resource) => resource === resourceType);
         
        if (selectedHandler) return true;

        return false;
   });

   if (handler === undefined) return [];

   return handler.actions;
}

export function getActionArguments(dictionary: IDictionary, actionName: string, resourceType: string): Array<any> {
   const handler = dictionary.handlers.find(handler => {
      const selectedHandler = handler.resources.find((resource) => resource === resourceType);
         
        if (selectedHandler) return true;

        return false;
   });

   const action = handler?.actions.find(action => {
      console.log(action)
      if (action.name === actionName) {
         console.log("a")
         return true;
      }else{
         return false;
      }
   })

   console.log(action)

   if (action !== undefined) {
      return action.parameters
   }

   return ['test'];
}





