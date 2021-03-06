package plugins.CeNo;

import freenet.keys.FreenetURI;
import freenet.pluginmanager.*;
import freenet.support.Logger;

import org.eclipse.jetty.server.Server;
import org.eclipse.jetty.server.ServerConnector;
import org.eclipse.jetty.server.handler.ContextHandler;
import org.eclipse.jetty.server.handler.ContextHandlerCollection;

import plugins.CeNo.FreenetInterface.HighLevelSimpleClientInterface;
import plugins.CeNo.FreenetInterface.NodeInterface;


public class CeNo implements FredPlugin, FredPluginVersioned, FredPluginRealVersioned {

	// Versions of the plugin, in human-readable and "real" format
	public static final String VERSION = "0.1.0";
	public static final int REAL_VERSION = 1;

	private PluginRespirator pluginRespirator;

	//Need to be read from config
	public static final Integer cacheLookupPort = 3091;
	public static final Integer cacheInsertPort = 3092;
	public static final Integer bundlerPort = 3093;

	private Server ceNoHttpServer;

	// Interface objects with fred
	private HighLevelSimpleClientInterface client;
	public static NodeInterface nodeInterface;

	// Plugin-specific configuration
	public static final String pluginUri = "/plugins/plugins.CeNo.CeNo";
	public static final String pluginName = "CeNo";
	public static Configuration initConfig;


	public void runPlugin(PluginRespirator pr)
	{        
		// Initialize interfaces with fred
		pluginRespirator = pr;
		client = new HighLevelSimpleClientInterface(pluginRespirator.getHLSimpleClient());
		nodeInterface = new NodeInterface(pluginRespirator.getNode());

		// Read properties of the configuration file
		initConfig = new Configuration();
		initConfig.readProperties();
		// If CeNo has no private key for inserting freesites,
		// generate a new key pair and store it in the configuration file
		if (initConfig.getProperty("insertURI") == null) {
			FreenetURI[] keyPair = nodeInterface.generateKeyPair();
			initConfig.setProperty("insertURI", keyPair[0].toString());
			initConfig.setProperty("requestURI", keyPair[1].toString());
			initConfig.storeProperties();
		}

		// Configure the CeNo's jetty embedded server
		ceNoHttpServer = new Server();
		configHttpServer(ceNoHttpServer);

		// Start server and wait until it gets interrupted
		try {
			ceNoHttpServer.start();
			ceNoHttpServer.join();
		} catch (Exception e) {
			e.printStackTrace();
		}
	}

	/**
	 * Configure CeNo's embedded server
	 * 
	 * @param ceNoHttpServer the jetty server to be configured
	 */
	private void configHttpServer(Server ceNoHttpServer) {     
		// Add a ServerConnector for each port
		ServerConnector httpConnector = new ServerConnector(ceNoHttpServer);
		httpConnector.setName("cacheLookup");
		httpConnector.setPort(cacheLookupPort);
		ServerConnector cacheConnector = new ServerConnector(ceNoHttpServer);
		cacheConnector.setName("cacheInsert");
		cacheConnector.setPort(cacheInsertPort);

		// Set server's connectors the ones configured above
		ceNoHttpServer.setConnectors(new ServerConnector[]{httpConnector, cacheConnector});

		// Create a collection of ContextHandlers for the server
		ContextHandlerCollection handlers = new ContextHandlerCollection();
		ceNoHttpServer.setHandler(handlers);

		// Configure ContextHandlers to listen to a specific port
		// and upon request call the appropriate AbstractHandler subclass	
		ContextHandler cacheLookupCtxHandler = new ContextHandler();
		cacheLookupCtxHandler.setHandler(new CacheLookupHandler());
		cacheLookupCtxHandler.setVirtualHosts(new String[]{"@cacheLookup"});
		ContextHandler cacheInsertCtxHandler = new ContextHandler();
		cacheInsertCtxHandler.setHandler(new CacheInsertHandler());
		cacheInsertCtxHandler.setVirtualHosts(new String[]{"@cacheInsert"});

		// Add the configured ContextHandlers to the server
		handlers.addHandler(cacheLookupCtxHandler);
		handlers.addHandler(cacheInsertCtxHandler);
	}

	public String getVersion() {
		return VERSION;
	}

	public long getRealVersion() {
		return REAL_VERSION;
	}

	/**
	 * Method called before termination of the CeNo plugin
	 * Terminates ceNoHttpServer and releases resources
	 */
	public void terminate()
	{
		// Stop ceNoHttpServer and unbind ports
		if (ceNoHttpServer != null) {
			try {
				ceNoHttpServer.stop();
			} catch (Exception e) {
				e.printStackTrace();
			}
		}
		Logger.normal(this, pluginName + " terminated.");
	}

}
