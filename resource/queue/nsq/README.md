## NSQ Driver

Implementation of NSQ. Connection will be made via NSQD using NSQ's tcp port (default: 4150). All topics and channels created will have `#ephemeral` suffix in order to be destroyed once there's no channel subscribed to the topic or no consumer is connected to the channel.

By default, the NSQ log is suppressed. There won't be any log from NSQ telling that the topic is missing or the NSQD cannot be reached. The reason behind this decission is because it's a bit annoying to see the status log of NSQ in regular interval.