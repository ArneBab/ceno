package plugins.CeNo.FreenetInterface;

import java.io.IOException;

import plugins.CeNo.HighLevelSimpleClientInterface;
import freenet.client.FetchException;
import freenet.client.FetchResult;
import freenet.client.InsertBlock;
import freenet.client.InsertContext;
import freenet.client.async.ClientPutter;
import freenet.keys.FreenetURI;
import freenet.keys.InsertableClientSSK;
import freenet.node.Node;
import freenet.node.RequestStarter;
import freenet.support.api.Bucket;
import freenet.support.api.RandomAccessBucket;

public class NodeInterface implements FreenetInterface {

	private Node node;
	private HighLevelSimpleClientInterface hl;

	public NodeInterface(Node node) {
		this.node = node;
	}

	public FetchResult fetchURI(FreenetURI uri) throws FetchException {
		return HighLevelSimpleClientInterface.fetchURI(uri);
	}

	/**
	 * Generate a new key pair for SSK insertions
	 * 
	 * @return FreenetURI array where 1st element is the insertURI and second element is the requestURI
	 */
	public FreenetURI[] generateKeyPair() {
		InsertableClientSSK key = InsertableClientSSK.createRandom(node.random, "");
		FreenetURI insertURI = key.getInsertURI();
		FreenetURI requestURI = key.getURI();
		return new FreenetURI[]{insertURI, requestURI};
	}

	public Bucket makeBucket(int length) throws IOException {
		return node.clientCore.persistentTempBucketFactory.makeBucket(length);
	}

	public boolean insertFreesite(FreenetURI insertURI, String content) throws IOException {
		RandomAccessBucket bucket = node.clientCore.persistentTempBucketFactory.makeBucket(content.length());
		bucket.getOutputStream().write(content.getBytes());
		bucket.setReadOnly();
		
		InsertBlock ib = new InsertBlock(bucket, null, insertURI);
		InsertContext ictx = hl.getInsertContext(true);
		ClientPutter pu = hl.insert(ib, false, null, false, ictx, RequestStarter.IMMEDIATE_SPLITFILE_PRIORITY_CLASS);
		return false;
	}

}